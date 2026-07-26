package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/cache"
	"dell-ecs-metrics-exporter/internal/config"
	"dell-ecs-metrics-exporter/internal/model"
	"dell-ecs-metrics-exporter/internal/profile"
)

func TestHealthAndVersionEndpoints(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	handler := testHandler(t, testOptions(t, now))

	recorder := perform(handler, http.MethodGet, "/health", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"UP"`) {
		t.Fatalf("liveness = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = perform(handler, http.MethodGet, "/api/v1/health", "")
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"status":"DOWN"`) {
		t.Fatalf("readiness = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = perform(handler, http.MethodGet, "/api/v1/version", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"supportedProfiles":["ecs-3.6","ecs-3.7","ecs-3.8.0","ecs-3.8.1"]`) ||
		!strings.Contains(recorder.Body.String(), `"sandboxCertifiedProfiles":[]`) {
		t.Fatalf("version = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestInventoryAuthenticationFilteringPaginationAndProblems(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	options := testOptions(t, now)
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("0123456789abcdef-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	options.Security = config.InventorySecurityConfig{
		Enabled: true, Authentication: "token", TokenFile: tokenPath, MaxPageSize: 2,
	}
	handler := testHandler(t, options)

	unauthorized := perform(handler, http.MethodGet, "/api/v1/buckets", "")
	if unauthorized.Code != http.StatusUnauthorized ||
		unauthorized.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unauthorized = %d %s", unauthorized.Code, unauthorized.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/buckets?cluster=alpha&size=1&sort=-name", nil)
	request.Header.Set("Authorization", "bearer 0123456789abcdef-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var page struct {
		Items []model.Bucket `json:"items"`
		Total int            `json:"totalElements"`
		Pages int            `json:"totalPages"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || page.Total != 2 || page.Pages != 2 ||
		len(page.Items) != 1 || page.Items[0].Name != "bucket-b" {
		t.Fatalf("page = %#v code=%d body=%s", page, recorder.Code, recorder.Body.String())
	}

	for _, path := range []string{
		"/api/v1/buckets?size=3",
		"/api/v1/buckets?unknown=x",
		"/api/v1/buckets?sort=owner",
		"/api/v1/buckets?page=-1",
	} {
		request = httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer 0123456789abcdef-token")
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s code = %d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestInventoryCollectionsAndSingleCluster(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	handler := testHandler(t, testOptions(t, now))
	for _, path := range []string{
		"/api/v1/clusters", "/api/v1/nodes?name=node-a",
		"/api/v1/namespaces?namespace=namespace-a",
		"/api/v1/replications?name=rg-a",
	} {
		recorder := perform(handler, http.MethodGet, path, "")
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"totalElements":1`) {
			t.Errorf("%s = %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	recorder := perform(handler, http.MethodGet, "/api/v1/clusters/alpha", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"name":"alpha"`) {
		t.Fatalf("single cluster = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = perform(handler, http.MethodGet, "/api/v1/clusters/missing", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing cluster code = %d", recorder.Code)
	}
	recorder = perform(handler, http.MethodPost, "/api/v1/buckets", "")
	if recorder.Code != http.StatusMethodNotAllowed ||
		recorder.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("POST = %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = perform(handler, http.MethodGet, "/api/v1/does-not-exist", "")
	if recorder.Code != http.StatusNotFound ||
		recorder.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unknown API = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestMetricsProtectionAndProxyAuthentication(t *testing.T) {
	t.Parallel()
	options := testOptions(t, time.Now())
	options.MetricsProtected = true
	options.Security = config.InventorySecurityConfig{
		Enabled: true, Authentication: "proxy", ProxyHeader: "X-Remote-User",
		TrustedProxyCIDRs: []string{"192.0.2.0/24"}, MaxPageSize: 10,
	}
	handler := testHandler(t, options)

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Remote-User", "monitor")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "metrics\n" {
		t.Fatalf("proxy metrics = %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("X-Remote-User", "monitor")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("untrusted proxy code = %d", recorder.Code)
	}
}

func TestHandlerRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	if _, err := NewHandler(Options{}); err == nil {
		t.Fatal("NewHandler accepted missing dependencies")
	}
	options := testOptions(t, time.Now())
	options.MetricsPath = "/api/v1"
	if _, err := NewHandler(options); err == nil {
		t.Fatal("NewHandler accepted the reserved API root as metrics path")
	}
	options = testOptions(t, time.Now())
	options.Security = config.InventorySecurityConfig{
		Enabled: true, Authentication: "token", TokenFile: filepath.Join(t.TempDir(), "missing"),
		MaxPageSize: 10,
	}
	if _, err := NewHandler(options); err == nil {
		t.Fatal("NewHandler accepted missing token file")
	}
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("invalid token value"), 0o600); err != nil {
		t.Fatal(err)
	}
	options.Security.TokenFile = tokenPath
	if _, err := NewHandler(options); err == nil {
		t.Fatal("NewHandler accepted whitespace in bearer token")
	}
}

func testOptions(t *testing.T, now time.Time) Options {
	t.Helper()
	store := cache.New()
	healthValue, total, used, available := 1.0, 1000.0, 250.0, 750.0
	store.ReplaceCluster("alpha", model.Cluster{
		Name: "alpha", Site: "dc-a", Environment: "test", VDC: "vdc-a",
		Health: &healthValue, TotalBytes: &total, UsedBytes: &used, AvailableBytes: &available,
		BucketCount: 2, NamespaceCount: 1, CollectedAt: now,
	})
	store.ReplaceNodes("alpha", []model.Node{{Cluster: "alpha", ID: "node-1", Name: "node-a", CollectedAt: now}})
	store.ReplaceNamespaces("alpha", []model.Namespace{{Cluster: "alpha", Name: "namespace-a", CollectedAt: now}})
	store.ReplaceBuckets("alpha", []model.Bucket{
		{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-a", CollectedAt: now},
		{Cluster: "alpha", Namespace: "namespace-a", Name: "bucket-b", CollectedAt: now},
	})
	store.ReplaceReplications("alpha", []model.Replication{{Cluster: "alpha", ID: "link-a", Group: "rg-a", CollectedAt: now}})
	return Options{
		Store: store, Catalog: loadCatalog(t), Metrics: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("metrics\n"))
		}),
		MetricsPath: "/metrics", Security: config.InventorySecurityConfig{MaxPageSize: 100},
		StaleTolerance: 15 * time.Minute, MaxStale: time.Hour,
		Build: BuildInfo{Version: "test", Commit: "abc", BuildDate: "today"},
		Now:   func() time.Time { return now },
	}
}

func testHandler(t *testing.T, options Options) *Handler {
	t.Helper()
	handler, err := NewHandler(options)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func loadCatalog(t *testing.T) *profile.Catalog {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	catalog, err := profile.LoadDir(filepath.Join(root, "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func perform(handler http.Handler, method, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
