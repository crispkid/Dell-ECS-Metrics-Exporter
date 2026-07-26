package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/health"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"
)

const contentTypeJSON = "application/json; charset=utf-8"

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

type Options struct {
	Store            *cache.Store
	Catalog          *profile.Catalog
	Metrics          http.Handler
	MetricsPath      string
	MetricsProtected bool
	Security         config.InventorySecurityConfig
	StaleTolerance   time.Duration
	MaxStale         time.Duration
	Build            BuildInfo
	Now              func() time.Time
}

type Handler struct {
	store          *cache.Store
	catalog        *profile.Catalog
	build          BuildInfo
	now            func() time.Time
	staleTolerance time.Duration
	maxStale       time.Duration
	maxPageSize    int
	auth           authenticator
	mux            *http.ServeMux
}

func NewHandler(options Options) (*Handler, error) {
	if options.Store == nil || options.Catalog == nil || options.Metrics == nil {
		return nil, fmt.Errorf("store, catalog, and metrics handler are required")
	}
	if options.Security.MaxPageSize < 1 || options.Security.MaxPageSize > 10000 {
		return nil, fmt.Errorf("max page size must be between 1 and 10000")
	}
	if options.StaleTolerance <= 0 || options.MaxStale < options.StaleTolerance {
		return nil, fmt.Errorf("max stale must be at least stale tolerance and both must be positive")
	}
	if !strings.HasPrefix(options.MetricsPath, "/") ||
		options.MetricsPath == "/" || strings.ContainsAny(options.MetricsPath, "?#") {
		return nil, fmt.Errorf("metrics path must be an absolute non-root path")
	}
	if options.MetricsPath == "/health" || options.MetricsPath == "/api/v1" ||
		strings.HasPrefix(options.MetricsPath, "/api/v1/") {
		return nil, fmt.Errorf("metrics path conflicts with a reserved API path")
	}
	if options.MetricsProtected && !options.Security.Enabled {
		return nil, fmt.Errorf("protected metrics require inventory authentication")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	auth, err := newAuthenticator(options.Security)
	if err != nil {
		return nil, err
	}
	handler := &Handler{
		store: options.Store, catalog: options.Catalog, build: normalizedBuild(options.Build),
		now: options.Now, staleTolerance: options.StaleTolerance, maxStale: options.MaxStale,
		maxPageSize: options.Security.MaxPageSize, auth: auth,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.liveness)
	mux.HandleFunc("/health", handler.methodNotAllowed)
	mux.HandleFunc("GET /api/v1/health", handler.readiness)
	mux.HandleFunc("/api/v1/health", handler.methodNotAllowed)
	mux.HandleFunc("GET /api/v1/version", handler.version)
	mux.HandleFunc("/api/v1/version", handler.methodNotAllowed)
	metricsHandler := options.Metrics
	if options.MetricsProtected {
		metricsHandler = handler.auth.require(metricsHandler)
	}
	mux.Handle("GET "+options.MetricsPath, metricsHandler)

	inventory := http.NewServeMux()
	inventory.HandleFunc("GET /api/v1/clusters", handler.clusters)
	inventory.HandleFunc("GET /api/v1/clusters/{cluster}", handler.cluster)
	inventory.HandleFunc("GET /api/v1/nodes", handler.nodes)
	inventory.HandleFunc("GET /api/v1/namespaces", handler.namespaces)
	inventory.HandleFunc("GET /api/v1/buckets", handler.buckets)
	inventory.HandleFunc("GET /api/v1/replications", handler.replications)
	inventory.HandleFunc("/", handler.apiNotFound)
	mux.Handle("/api/v1/", handler.auth.require(handler.methodGuard(inventory)))
	handler.mux = mux
	return handler, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	h.mux.ServeHTTP(writer, request)
}

func (h *Handler) liveness(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"status": health.UP, "component": "process"})
}

func (h *Handler) readiness(writer http.ResponseWriter, _ *http.Request) {
	report := health.Evaluate(
		h.store.Snapshot(), h.staleTolerance, h.maxStale, h.now().UTC(),
	)
	status := http.StatusOK
	if report.Status == health.DOWN {
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, report)
}

func (h *Handler) version(writer http.ResponseWriter, _ *http.Request) {
	certified := make([]string, 0)
	supported := make([]string, 0, len(h.catalog.Profiles()))
	for _, value := range h.catalog.Profiles() {
		supported = append(supported, value.ProfileID)
		if value.Evidence.SandboxCertified {
			certified = append(certified, value.ProfileID)
		}
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"build": h.build, "supportedProfiles": supported,
		"sandboxCertifiedProfiles": certified,
	})
}

func (h *Handler) methodNotAllowed(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Allow", http.MethodGet)
	writeProblem(
		writer, request, http.StatusMethodNotAllowed, "method not allowed",
		"only GET is supported for this resource",
	)
}

func (h *Handler) methodGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			h.methodNotAllowed(writer, request)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (h *Handler) apiNotFound(writer http.ResponseWriter, request *http.Request) {
	writeProblem(writer, request, http.StatusNotFound, "resource not found", "the requested API resource does not exist")
}

func (h *Handler) clusters(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, map[string]bool{"page": true, "size": true, "sort": true, "name": true}) {
		return
	}
	items := h.store.Snapshot().Clusters
	name := request.URL.Query().Get("name")
	items = filter(items, func(value model.Cluster) bool { return name == "" || value.Name == name })
	sortBy(request.URL.Query().Get("sort"), items, func(value model.Cluster) (string, string, string) {
		return value.Name, value.Name, ""
	})
	writePage(h, writer, request, items, clusterCollectedAt)
}

func (h *Handler) cluster(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, map[string]bool{}) {
		return
	}
	name := request.PathValue("cluster")
	if name == "" {
		writeProblem(writer, request, http.StatusBadRequest, "invalid cluster", "cluster path parameter is invalid")
		return
	}
	for _, value := range h.store.Snapshot().Clusters {
		if value.Name == name {
			writeJSON(writer, http.StatusOK, value)
			return
		}
	}
	writeProblem(writer, request, http.StatusNotFound, "cluster not found", "the requested cluster is not cached")
}

func (h *Handler) nodes(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, map[string]bool{"page": true, "size": true, "sort": true, "cluster": true, "name": true}) {
		return
	}
	query := request.URL.Query()
	items := h.store.Snapshot().Nodes
	items = filter(items, func(value model.Node) bool {
		return matches(query.Get("cluster"), value.Cluster) && matches(query.Get("name"), value.Name)
	})
	sortBy(query.Get("sort"), items, func(value model.Node) (string, string, string) {
		return value.Name, value.Cluster, ""
	})
	writePage(h, writer, request, items, nodeCollectedAt)
}

func (h *Handler) namespaces(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, inventoryQueryFields()) {
		return
	}
	query := request.URL.Query()
	items := h.store.Snapshot().Namespaces
	items = filter(items, func(value model.Namespace) bool {
		return matches(query.Get("cluster"), value.Cluster) &&
			matches(query.Get("namespace"), value.Name) && matches(query.Get("name"), value.Name)
	})
	sortBy(query.Get("sort"), items, func(value model.Namespace) (string, string, string) {
		return value.Name, value.Cluster, value.Name
	})
	writePage(h, writer, request, items, namespaceCollectedAt)
}

func (h *Handler) buckets(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, inventoryQueryFields()) {
		return
	}
	query := request.URL.Query()
	items := h.store.Snapshot().Buckets
	items = filter(items, func(value model.Bucket) bool {
		return matches(query.Get("cluster"), value.Cluster) &&
			matches(query.Get("namespace"), value.Namespace) && matches(query.Get("name"), value.Name)
	})
	sortBy(query.Get("sort"), items, func(value model.Bucket) (string, string, string) {
		return value.Name, value.Cluster, value.Namespace
	})
	writePage(h, writer, request, items, bucketCollectedAt)
}

func (h *Handler) replications(writer http.ResponseWriter, request *http.Request) {
	if !validateQuery(writer, request, map[string]bool{"page": true, "size": true, "sort": true, "cluster": true, "name": true}) {
		return
	}
	query := request.URL.Query()
	items := h.store.Snapshot().Replications
	items = filter(items, func(value model.Replication) bool {
		return matches(query.Get("cluster"), value.Cluster) && matches(query.Get("name"), value.Group)
	})
	sortBy(query.Get("sort"), items, func(value model.Replication) (string, string, string) {
		return value.Group, value.Cluster, ""
	})
	writePage(h, writer, request, items, replicationCollectedAt)
}

func writePage[T any](
	h *Handler,
	writer http.ResponseWriter,
	request *http.Request,
	items []T,
	collectedAt func(T) time.Time,
) {
	page, size, err := pagination(request.URL.Query(), h.maxPageSize)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid pagination", err.Error())
		return
	}
	total := len(items)
	start := page * size
	if start > total {
		start = total
	}
	end := min(start+size, total)
	var newest time.Time
	for _, item := range items {
		if timestamp := collectedAt(item); timestamp.After(newest) {
			newest = timestamp
		}
	}
	totalPages := 0
	if total != 0 {
		totalPages = (total + size - 1) / size
	}
	var newestValue any
	if !newest.IsZero() {
		newestValue = newest
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"items": items[start:end], "page": page, "size": size,
		"totalElements": total, "totalPages": totalPages, "collectedAt": newestValue,
	})
}

type authenticator struct {
	enabled      bool
	method       string
	tokenFile    string
	proxyHeader  string
	trustedCIDRs []*net.IPNet
}

func newAuthenticator(settings config.InventorySecurityConfig) (authenticator, error) {
	result := authenticator{
		enabled: settings.Enabled, method: settings.Authentication,
		tokenFile: settings.TokenFile, proxyHeader: settings.ProxyHeader,
	}
	if !result.enabled {
		return result, nil
	}
	switch result.method {
	case "token":
		if _, err := readToken(result.tokenFile); err != nil {
			return authenticator{}, err
		}
	case "proxy":
		for _, value := range settings.TrustedProxyCIDRs {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return authenticator{}, fmt.Errorf("parse trusted proxy CIDR: %w", err)
			}
			result.trustedCIDRs = append(result.trustedCIDRs, network)
		}
		if result.proxyHeader == "" || strings.ContainsAny(result.proxyHeader, ":\r\n") ||
			len(result.trustedCIDRs) == 0 {
			return authenticator{}, fmt.Errorf("proxy authentication configuration is invalid")
		}
	default:
		return authenticator{}, fmt.Errorf("inventory authentication method is unsupported")
	}
	return result, nil
}

func (a authenticator) require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !a.enabled {
			next.ServeHTTP(writer, request)
			return
		}
		authorized := false
		switch a.method {
		case "token":
			expected, err := readToken(a.tokenFile)
			provided := bearerToken(request.Header.Get("Authorization"))
			authorized = err == nil && len(provided) == len(expected) &&
				subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
		case "proxy":
			host, _, err := net.SplitHostPort(request.RemoteAddr)
			ip := net.ParseIP(host)
			trusted := err == nil && slices.ContainsFunc(a.trustedCIDRs, func(network *net.IPNet) bool {
				return network.Contains(ip)
			})
			authorized = trusted && strings.TrimSpace(request.Header.Get(a.proxyHeader)) != ""
		}
		if !authorized {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "valid inventory API authentication is required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func bearerToken(value string) string {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func readToken(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read inventory token file: %w", err)
	}
	token := strings.TrimRight(string(content), "\r\n")
	if len(token) < 16 || len(token) > 4096 || strings.ContainsFunc(token, unicode.IsSpace) {
		return "", fmt.Errorf("inventory token file must contain 16 to 4096 non-whitespace characters")
	}
	return token, nil
}

func pagination(query url.Values, maxSize int) (int, int, error) {
	page, size := 0, min(100, maxSize)
	var err error
	if raw := query.Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 0 {
			return 0, 0, fmt.Errorf("page must be a non-negative integer")
		}
	}
	if raw := query.Get("size"); raw != "" {
		size, err = strconv.Atoi(raw)
		if err != nil || size < 1 || size > maxSize {
			return 0, 0, fmt.Errorf("size must be between 1 and %d", maxSize)
		}
	}
	if page > 1_000_000 || (size != 0 && page > int(^uint(0)>>1)/size) {
		return 0, 0, fmt.Errorf("page is too large")
	}
	return page, size, nil
}

func validateQuery(writer http.ResponseWriter, request *http.Request, allowed map[string]bool) bool {
	query := request.URL.Query()
	for name, values := range query {
		if !allowed[name] || len(values) != 1 {
			writeProblem(writer, request, http.StatusBadRequest, "invalid query", "query contains an unsupported or repeated parameter")
			return false
		}
		if len(values[0]) > 256 {
			writeProblem(writer, request, http.StatusBadRequest, "invalid query", "query parameter value exceeds 256 characters")
			return false
		}
	}
	sort := strings.TrimPrefix(query.Get("sort"), "-")
	if sort != "" && sort != "name" && sort != "cluster" && sort != "namespace" {
		writeProblem(writer, request, http.StatusBadRequest, "invalid sort", "sort must be name, cluster, or namespace")
		return false
	}
	return true
}

func sortBy[T any](sortValue string, items []T, keys func(T) (string, string, string)) {
	descending := strings.HasPrefix(sortValue, "-")
	field := strings.TrimPrefix(sortValue, "-")
	if field == "" {
		field = "name"
	}
	slices.SortStableFunc(items, func(left, right T) int {
		leftName, leftCluster, leftNamespace := keys(left)
		rightName, rightCluster, rightNamespace := keys(right)
		var comparison int
		switch field {
		case "cluster":
			comparison = strings.Compare(leftCluster, rightCluster)
		case "namespace":
			comparison = strings.Compare(leftNamespace, rightNamespace)
		default:
			comparison = strings.Compare(leftName, rightName)
		}
		if comparison == 0 {
			comparison = strings.Compare(leftCluster+"\x00"+leftNamespace+"\x00"+leftName, rightCluster+"\x00"+rightNamespace+"\x00"+rightName)
		}
		if descending {
			return -comparison
		}
		return comparison
	})
}

func filter[T any](values []T, include func(T) bool) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		if include(value) {
			result = append(result, value)
		}
	}
	return result
}

func matches(query, value string) bool {
	return query == "" || query == value
}

func inventoryQueryFields() map[string]bool {
	return map[string]bool{
		"cluster": true, "namespace": true, "name": true,
		"page": true, "size": true, "sort": true,
	}
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, title, detail string) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "about:blank", "title": title, "status": status,
		"detail": detail, "instance": request.URL.Path,
	})
}

func normalizedBuild(build BuildInfo) BuildInfo {
	if build.Version == "" {
		build.Version = "dev"
	}
	if build.Commit == "" {
		build.Commit = "unknown"
	}
	if build.BuildDate == "" {
		build.BuildDate = "unknown"
	}
	return build
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", contentTypeJSON)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func clusterCollectedAt(value model.Cluster) time.Time         { return value.CollectedAt }
func nodeCollectedAt(value model.Node) time.Time               { return value.CollectedAt }
func namespaceCollectedAt(value model.Namespace) time.Time     { return value.CollectedAt }
func bucketCollectedAt(value model.Bucket) time.Time           { return value.CollectedAt }
func replicationCollectedAt(value model.Replication) time.Time { return value.CollectedAt }
