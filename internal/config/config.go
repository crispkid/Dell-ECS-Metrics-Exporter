package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var environmentReference = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(value []byte) error {
	parsed, err := time.ParseDuration(string(value))
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Cache      CacheConfig      `yaml:"cache"`
	Collector  CollectorConfig  `yaml:"collector"`
	Security   SecurityConfig   `yaml:"security"`
	ECS        ECSConfig        `yaml:"ecs"`
}

type ServerConfig struct {
	ListenAddress string          `yaml:"listenAddress"`
	TLS           ServerTLSConfig `yaml:"tls"`
}

type ServerTLSConfig struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

type PrometheusConfig struct {
	Path      string `yaml:"path"`
	Protected bool   `yaml:"protected"`
}

type CacheConfig struct {
	StaleTolerance Duration `yaml:"staleTolerance"`
	MaxStale       Duration `yaml:"maxStale"`
}

type CollectorConfig struct {
	DefaultTimeout Duration          `yaml:"defaultTimeout"`
	Retry          RetryConfig       `yaml:"retry"`
	RateLimit      RateLimitConfig   `yaml:"rateLimit"`
	Intervals      IntervalConfig    `yaml:"intervals"`
	Concurrency    ConcurrencyConfig `yaml:"concurrency"`
	JitterRatio    float64           `yaml:"jitterRatio"`
}

type RetryConfig struct {
	MaxAttempts    int      `yaml:"maxAttempts"`
	InitialBackoff Duration `yaml:"initialBackoff"`
	MaxBackoff     Duration `yaml:"maxBackoff"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requestsPerSecond"`
	Burst             int     `yaml:"burst"`
}

type IntervalConfig struct {
	Cluster     Duration `yaml:"cluster"`
	Node        Duration `yaml:"node"`
	Performance Duration `yaml:"performance"`
	Replication Duration `yaml:"replication"`
	Recovery    Duration `yaml:"recovery"`
	Namespace   Duration `yaml:"namespace"`
	Bucket      Duration `yaml:"bucket"`
}

func (c IntervalConfig) Values() map[string]time.Duration {
	return map[string]time.Duration{
		"cluster":     c.Cluster.Duration,
		"node":        c.Node.Duration,
		"performance": c.Performance.Duration,
		"replication": c.Replication.Duration,
		"recovery":    c.Recovery.Duration,
		"namespace":   c.Namespace.Duration,
		"bucket":      c.Bucket.Duration,
	}
}

type ConcurrencyConfig struct {
	MaxConcurrentRequestsPerCluster int `yaml:"maxConcurrentRequestsPerCluster"`
	BucketPageSize                  int `yaml:"bucketPageSize"`
}

type SecurityConfig struct {
	InventoryAPI InventorySecurityConfig `yaml:"inventoryApi"`
}

type InventorySecurityConfig struct {
	Enabled           bool     `yaml:"enabled"`
	Authentication    string   `yaml:"authentication"`
	TokenFile         string   `yaml:"tokenFile"`
	ProxyHeader       string   `yaml:"proxyHeader"`
	TrustedProxyCIDRs []string `yaml:"trustedProxyCIDRs"`
	MaxPageSize       int      `yaml:"maxPageSize"`
}

type ECSConfig struct {
	Clusters []ClusterConfig `yaml:"clusters"`
}

type ClusterConfig struct {
	Name          string             `yaml:"name"`
	Site          string             `yaml:"site"`
	Environment   string             `yaml:"environment"`
	Endpoint      string             `yaml:"endpoint"`
	Username      string             `yaml:"username"`
	UsernameFile  string             `yaml:"usernameFile"`
	Password      string             `yaml:"password"`
	PasswordFile  string             `yaml:"passwordFile"`
	TLS           ClusterTLSConfig   `yaml:"tls"`
	Timeouts      TimeoutOverride    `yaml:"timeouts"`
	Intervals     IntervalOverride   `yaml:"intervals"`
	Capabilities  CapabilityConfig   `yaml:"capabilities"`
	NodeResources NodeResourceConfig `yaml:"nodeResources"`
	Replication   ReplicationTargets `yaml:"replication"`
}

type CapabilityConfig struct {
	EnabledConditional []string `yaml:"enabledConditional"`
}

type NodeResourceConfig struct {
	Filesystems          []string `yaml:"filesystems"`
	NetworkInterfaces    []string `yaml:"networkInterfaces"`
	MaxNetworkInterfaces int      `yaml:"maxNetworkInterfaces"`
	PreferBondInterfaces *bool    `yaml:"preferBondInterfaces"`
}

func (c NodeResourceConfig) EffectiveMaxNetworkInterfaces() int {
	if c.MaxNetworkInterfaces == 0 {
		return 16
	}
	return c.MaxNetworkInterfaces
}

func (c NodeResourceConfig) BondPreferenceEnabled() bool {
	return c.PreferBondInterfaces == nil || *c.PreferBondInterfaces
}

type ClusterTLSConfig struct {
	Verify *bool  `yaml:"verify"`
	CAFile string `yaml:"caFile"`
}

func (c ClusterTLSConfig) VerificationEnabled() bool {
	return c.Verify == nil || *c.Verify
}

type IntervalOverride struct {
	Cluster     *Duration `yaml:"cluster"`
	Node        *Duration `yaml:"node"`
	Performance *Duration `yaml:"performance"`
	Replication *Duration `yaml:"replication"`
	Recovery    *Duration `yaml:"recovery"`
	Namespace   *Duration `yaml:"namespace"`
	Bucket      *Duration `yaml:"bucket"`
}

type TimeoutOverride struct {
	Connect *Duration `yaml:"connect"`
	Read    *Duration `yaml:"read"`
	Overall *Duration `yaml:"overall"`
}

func (o TimeoutOverride) Resolve(defaultOverall time.Duration) (time.Duration, time.Duration, time.Duration) {
	overall := defaultOverall
	if o.Overall != nil {
		overall = o.Overall.Duration
	}
	connect := min(overall, 10*time.Second)
	if o.Connect != nil {
		connect = o.Connect.Duration
	}
	read := overall
	if o.Read != nil {
		read = o.Read.Duration
	}
	return connect, read, overall
}

func (o IntervalOverride) Resolve(defaults IntervalConfig) map[string]time.Duration {
	result := defaults.Values()
	overrides := map[string]*Duration{
		"cluster": o.Cluster, "node": o.Node, "performance": o.Performance,
		"replication": o.Replication, "recovery": o.Recovery,
		"namespace": o.Namespace, "bucket": o.Bucket,
	}
	for name, value := range overrides {
		if value != nil {
			result[name] = value.Duration
		}
	}
	return result
}

type ReplicationTargets struct {
	Groups []string `yaml:"groups"`
	Links  []string `yaml:"links"`
}

func Defaults() Config {
	return Config{
		Server:     ServerConfig{ListenAddress: ":8080"},
		Prometheus: PrometheusConfig{Path: "/metrics"},
		Cache: CacheConfig{
			StaleTolerance: Duration{Duration: 15 * time.Minute},
			MaxStale:       Duration{Duration: time.Hour},
		},
		Collector: CollectorConfig{
			DefaultTimeout: Duration{Duration: 30 * time.Second},
			Retry: RetryConfig{
				MaxAttempts:    3,
				InitialBackoff: Duration{Duration: time.Second},
				MaxBackoff:     Duration{Duration: 10 * time.Second},
			},
			RateLimit: RateLimitConfig{RequestsPerSecond: 10, Burst: 4},
			Intervals: IntervalConfig{
				Cluster: Duration{Duration: time.Minute}, Node: Duration{Duration: time.Minute},
				Performance: Duration{Duration: time.Minute},
				Replication: Duration{Duration: 2 * time.Minute},
				Recovery:    Duration{Duration: 2 * time.Minute},
				Namespace:   Duration{Duration: 5 * time.Minute},
				Bucket:      Duration{Duration: 5 * time.Minute},
			},
			Concurrency: ConcurrencyConfig{
				MaxConcurrentRequestsPerCluster: 4,
				BucketPageSize:                  500,
			},
			JitterRatio: 0.1,
		},
		Security: SecurityConfig{InventoryAPI: InventorySecurityConfig{
			Enabled: true, Authentication: "token", MaxPageSize: 500,
			ProxyHeader: "X-Remote-User",
		}},
	}
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	expanded, err := expandEnvironment(content, os.LookupEnv)
	if err != nil {
		return Config{}, err
	}
	value := Defaults()
	decoder := yaml.NewDecoder(bytes.NewReader(expanded))
	decoder.KnownFields(true)
	if err := decoder.Decode(&value); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("configuration contains multiple YAML documents")
		}
		return Config{}, fmt.Errorf("decode trailing configuration: %w", err)
	}
	if err := value.ApplyEnvironment(); err != nil {
		return Config{}, err
	}
	if err := value.Validate(); err != nil {
		return Config{}, err
	}
	return value, nil
}

func (c *Config) ApplyEnvironment() error {
	if value := os.Getenv("ECS_EXPORTER_LISTEN_ADDRESS"); value != "" {
		c.Server.ListenAddress = value
	}
	if value := os.Getenv("ECS_EXPORTER_INVENTORY_TOKEN_FILE"); value != "" {
		c.Security.InventoryAPI.TokenFile = value
	}
	if value := os.Getenv("ECS_EXPORTER_MAX_PAGE_SIZE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("ECS_EXPORTER_MAX_PAGE_SIZE: %w", err)
		}
		c.Security.InventoryAPI.MaxPageSize = parsed
	}
	return nil
}

func (c Config) Validate() error {
	var problems []error
	if c.Server.ListenAddress == "" {
		problems = append(problems, fmt.Errorf("server.listenAddress is required"))
	} else if _, err := net.ResolveTCPAddr("tcp", c.Server.ListenAddress); err != nil {
		problems = append(problems, fmt.Errorf("server.listenAddress: %w", err))
	}
	if (c.Server.TLS.CertFile == "") != (c.Server.TLS.KeyFile == "") {
		problems = append(problems, fmt.Errorf("server TLS certFile and keyFile must be set together"))
	} else if c.Server.TLS.CertFile != "" {
		problems = append(problems, validateReadableFile(c.Server.TLS.CertFile, "server TLS certificate")...)
		problems = append(problems, validateReadableFile(c.Server.TLS.KeyFile, "server TLS key")...)
	}
	if !validHTTPPath(c.Prometheus.Path) {
		problems = append(problems, fmt.Errorf("prometheus.path must be an absolute path"))
	} else if c.Prometheus.Path == "/health" || c.Prometheus.Path == "/api/v1" ||
		strings.HasPrefix(c.Prometheus.Path, "/api/v1/") {
		problems = append(problems, fmt.Errorf("prometheus.path conflicts with a reserved API path"))
	}
	if c.Prometheus.Protected && !c.Security.InventoryAPI.Enabled {
		problems = append(problems, fmt.Errorf("protected Prometheus endpoint requires inventory API authentication"))
	}
	if c.Cache.StaleTolerance.Duration <= 0 ||
		c.Cache.MaxStale.Duration < c.Cache.StaleTolerance.Duration {
		problems = append(problems, fmt.Errorf("cache maxStale must be at least staleTolerance and both must be positive"))
	}
	if c.Collector.DefaultTimeout.Duration <= 0 {
		problems = append(problems, fmt.Errorf("collector.defaultTimeout must be positive"))
	}
	if c.Collector.Retry.MaxAttempts < 1 || c.Collector.Retry.MaxAttempts > 10 {
		problems = append(problems, fmt.Errorf("collector.retry.maxAttempts must be between 1 and 10"))
	}
	if c.Collector.Retry.InitialBackoff.Duration <= 0 ||
		c.Collector.Retry.MaxBackoff.Duration < c.Collector.Retry.InitialBackoff.Duration {
		problems = append(problems, fmt.Errorf("collector retry backoff range is invalid"))
	}
	if c.Collector.RateLimit.RequestsPerSecond <= 0 ||
		c.Collector.RateLimit.RequestsPerSecond > 10000 {
		problems = append(problems, fmt.Errorf(
			"collector.rateLimit.requestsPerSecond must be greater than 0 and at most 10000",
		))
	}
	if c.Collector.RateLimit.Burst < 1 || c.Collector.RateLimit.Burst > 10000 {
		problems = append(problems, fmt.Errorf("collector.rateLimit.burst must be between 1 and 10000"))
	}
	if c.Collector.JitterRatio < 0 || c.Collector.JitterRatio > 0.5 {
		problems = append(problems, fmt.Errorf("collector.jitterRatio must be between 0 and 0.5"))
	}
	for name, interval := range c.Collector.Intervals.Values() {
		if interval <= 0 {
			problems = append(problems, fmt.Errorf("collector.intervals.%s must be positive", name))
		}
	}
	if c.Collector.Concurrency.MaxConcurrentRequestsPerCluster < 1 ||
		c.Collector.Concurrency.MaxConcurrentRequestsPerCluster > 64 {
		problems = append(problems, fmt.Errorf("collector concurrency must be between 1 and 64"))
	}
	if c.Collector.Concurrency.BucketPageSize < 1 ||
		c.Collector.Concurrency.BucketPageSize > 10000 {
		problems = append(problems, fmt.Errorf("collector bucketPageSize must be between 1 and 10000"))
	}
	problems = append(problems, c.Security.InventoryAPI.validate()...)
	if len(c.ECS.Clusters) == 0 {
		problems = append(problems, fmt.Errorf("ecs.clusters must contain at least one cluster"))
	}
	seen := make(map[string]struct{}, len(c.ECS.Clusters))
	for index := range c.ECS.Clusters {
		cluster := &c.ECS.Clusters[index]
		if _, exists := seen[cluster.Name]; exists {
			problems = append(problems, fmt.Errorf("ecs cluster name %q is duplicated", cluster.Name))
		}
		seen[cluster.Name] = struct{}{}
		problems = append(problems, cluster.validate()...)
		if _, _, err := cluster.Credentials(); err != nil {
			problems = append(problems, err)
		}
		for name, interval := range cluster.Intervals.Resolve(c.Collector.Intervals) {
			if interval <= 0 {
				problems = append(problems, fmt.Errorf("cluster %q interval %s must be positive", cluster.Name, name))
			}
		}
		connect, read, overall := cluster.Timeouts.Resolve(c.Collector.DefaultTimeout.Duration)
		if connect <= 0 || read <= 0 || overall <= 0 ||
			connect > overall || read > overall {
			problems = append(problems, fmt.Errorf(
				"cluster %q timeouts must be positive and connect/read cannot exceed overall",
				cluster.Name,
			))
		}
	}
	return errors.Join(problems...)
}

func (s InventorySecurityConfig) validate() []error {
	var problems []error
	if s.MaxPageSize < 1 || s.MaxPageSize > 10000 {
		problems = append(problems, fmt.Errorf("security.inventoryApi.maxPageSize must be between 1 and 10000"))
	}
	if !s.Enabled {
		return problems
	}
	switch s.Authentication {
	case "token":
		if s.TokenFile == "" {
			problems = append(problems, fmt.Errorf("inventory token authentication requires tokenFile"))
		} else {
			content, err := os.ReadFile(filepath.Clean(s.TokenFile))
			if err != nil {
				problems = append(problems, fmt.Errorf("read inventory token file: %w", err))
			} else if token := trimLineEndings(string(content)); len(token) < 16 ||
				len(token) > 4096 || strings.ContainsFunc(token, unicode.IsSpace) {
				problems = append(problems, fmt.Errorf(
					"inventory token file must contain 16 to 4096 non-whitespace characters",
				))
			}
		}
	case "proxy":
		if s.ProxyHeader == "" || len(s.TrustedProxyCIDRs) == 0 {
			problems = append(problems, fmt.Errorf("proxy authentication requires proxyHeader and trustedProxyCIDRs"))
		}
		for _, value := range s.TrustedProxyCIDRs {
			if _, _, err := net.ParseCIDR(value); err != nil {
				problems = append(problems, fmt.Errorf("trusted proxy CIDR %q: %w", value, err))
			}
		}
	default:
		problems = append(problems, fmt.Errorf("inventory authentication must be token or proxy"))
	}
	return problems
}

func (c ClusterConfig) validate() []error {
	var problems []error
	if !validIdentifier(c.Name, 128) {
		problems = append(problems, fmt.Errorf("ecs cluster name must be a trimmed, printable value of 1 to 128 bytes"))
	}
	if !validIdentifier(c.Site, 128) {
		problems = append(problems, fmt.Errorf("cluster %q site must be a trimmed, printable value of 1 to 128 bytes", c.Name))
	}
	if !validIdentifier(c.Environment, 64) {
		problems = append(problems, fmt.Errorf("cluster %q environment must be a trimmed, printable value of 1 to 64 bytes", c.Name))
	}
	parsed, err := url.Parse(c.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		problems = append(problems, fmt.Errorf("cluster %q endpoint must be an origin-only HTTPS URL", c.Name))
	}
	if (c.Username == "") == (c.UsernameFile == "") {
		problems = append(problems, fmt.Errorf("cluster %q requires exactly one of username or usernameFile", c.Name))
	}
	if (c.Password == "") == (c.PasswordFile == "") {
		problems = append(problems, fmt.Errorf("cluster %q requires exactly one of password or passwordFile", c.Name))
	}
	if c.TLS.CAFile != "" {
		problems = append(problems, validateReadableFile(c.TLS.CAFile, "cluster CA file")...)
	}
	allowedConditional := map[string]bool{
		"namespace_performance": true, "node_disk_capacity": true,
		"node_service_process": true,
		"recovery_progress":    true, "vdc_performance": true,
	}
	seenConditional := make(map[string]struct{}, len(c.Capabilities.EnabledConditional))
	for _, capability := range c.Capabilities.EnabledConditional {
		if !allowedConditional[capability] {
			problems = append(problems, fmt.Errorf(
				"cluster %q conditional capability %q is unsupported", c.Name, capability,
			))
		}
		if _, exists := seenConditional[capability]; exists {
			problems = append(problems, fmt.Errorf(
				"cluster %q conditional capability %q is duplicated", c.Name, capability,
			))
		}
		seenConditional[capability] = struct{}{}
	}
	if c.NodeResources.MaxNetworkInterfaces < 0 ||
		c.NodeResources.MaxNetworkInterfaces > 128 {
		problems = append(problems, fmt.Errorf(
			"cluster %q nodeResources.maxNetworkInterfaces must be 0/default or between 1 and 128",
			c.Name,
		))
	}
	for field, values := range map[string][]string{
		"filesystem":        c.NodeResources.Filesystems,
		"network interface": c.NodeResources.NetworkInterfaces,
	} {
		seenValues := make(map[string]struct{}, len(values))
		for _, value := range values {
			if !validIdentifier(value, 128) {
				problems = append(problems, fmt.Errorf(
					"cluster %q node resource %s is invalid", c.Name, field,
				))
				continue
			}
			if _, exists := seenValues[value]; exists {
				problems = append(problems, fmt.Errorf(
					"cluster %q node resource %s %q is duplicated", c.Name, field, value,
				))
			}
			seenValues[value] = struct{}{}
		}
	}
	if slices.Contains(c.Capabilities.EnabledConditional, "node_disk_capacity") &&
		len(c.NodeResources.Filesystems) == 0 {
		problems = append(problems, fmt.Errorf(
			"cluster %q conditional node_disk_capacity requires nodeResources.filesystems", c.Name,
		))
	}
	for kind, values := range map[string][]string{
		"replication group": c.Replication.Groups,
		"replication link":  c.Replication.Links,
	} {
		seen := make(map[string]struct{}, len(values))
		for _, value := range values {
			if !validIdentifier(value, 512) {
				problems = append(problems, fmt.Errorf("cluster %q %s id is invalid", c.Name, kind))
				continue
			}
			if _, exists := seen[value]; exists {
				problems = append(problems, fmt.Errorf("cluster %q %s id %q is duplicated", c.Name, kind, value))
			}
			seen[value] = struct{}{}
		}
	}
	groupIDs := make(map[string]struct{}, len(c.Replication.Groups))
	for _, value := range c.Replication.Groups {
		groupIDs[value] = struct{}{}
	}
	for _, value := range c.Replication.Links {
		if _, exists := groupIDs[value]; exists {
			problems = append(problems, fmt.Errorf(
				"cluster %q replication id %q is duplicated across groups and links",
				c.Name, value,
			))
		}
	}
	return problems
}

func (c ClusterConfig) Credentials() (string, string, error) {
	username, err := resolveSecret(c.Username, c.UsernameFile)
	if err != nil {
		return "", "", fmt.Errorf("cluster %q username: %w", c.Name, err)
	}
	password, err := resolveSecret(c.Password, c.PasswordFile)
	if err != nil {
		return "", "", fmt.Errorf("cluster %q password: %w", c.Name, err)
	}
	if !validIdentifier(username, 256) {
		return "", "", fmt.Errorf(
			"cluster %q username must be a trimmed, printable value of 1 to 256 bytes",
			c.Name,
		)
	}
	if strings.Contains(username, ":") || containsControl(username) {
		return "", "", fmt.Errorf("cluster %q username cannot contain a colon or control characters", c.Name)
	}
	if password == "" || len(password) > 4096 {
		return "", "", fmt.Errorf("cluster %q password must contain 1 to 4096 characters", c.Name)
	}
	return username, password, nil
}

func resolveSecret(value, path string) (string, error) {
	if path == "" {
		if value == "" {
			return "", fmt.Errorf("secret is empty")
		}
		return value, nil
	}
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}
	resolved := trimLineEndings(string(content))
	if resolved == "" {
		return "", fmt.Errorf("secret file is empty")
	}
	return resolved, nil
}

func trimLineEndings(value string) string {
	return strings.TrimRight(value, "\r\n")
}

func expandEnvironment(input []byte, lookup func(string) (string, bool)) ([]byte, error) {
	var missing []string
	expanded := environmentReference.ReplaceAllStringFunc(string(input), func(reference string) string {
		name := environmentReference.FindStringSubmatch(reference)[1]
		value, found := lookup(name)
		if !found {
			missing = append(missing, name)
			return ""
		}
		return value
	})
	if len(missing) != 0 {
		return nil, fmt.Errorf("configuration references unset environment variables: %s", strings.Join(missing, ", "))
	}
	return []byte(expanded), nil
}

func validHTTPPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "?#") && value != "/"
}

func validIdentifier(value string, maxBytes int) bool {
	return value != "" && value == strings.TrimSpace(value) &&
		len(value) <= maxBytes && utf8.ValidString(value) && !containsControl(value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validateReadableFile(path, description string) []error {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return []error{fmt.Errorf("%s: %w", description, err)}
	}
	if !info.Mode().IsRegular() {
		return []error{fmt.Errorf("%s must be a regular file", description)}
	}
	return nil
}
