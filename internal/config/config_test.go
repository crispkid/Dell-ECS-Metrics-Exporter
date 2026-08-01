package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadValidConfigurationAndSecrets(t *testing.T) {
	t.Setenv("TEST_ECS_USERNAME", "monitor")
	t.Setenv("ECS_EXPORTER_LISTEN_ADDRESS", "127.0.0.1:18080")
	t.Setenv("ECS_EXPORTER_MAX_PAGE_SIZE", "250")
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "password")
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(passwordFile, []byte("secret-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenFile, []byte("0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := `
security:
  inventoryApi:
    tokenFile: ` + tokenFile + `
ecs:
  clusters:
    - name: test
      site: lab
      environment: test
      endpoint: https://ecs.example.invalid
      username: ${TEST_ECS_USERNAME}
      passwordFile: ` + passwordFile + `
      tls:
        verify: false
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if value.Server.ListenAddress != "127.0.0.1:18080" ||
		value.Security.InventoryAPI.MaxPageSize != 250 ||
		value.Collector.Intervals.Bucket.Duration != 5*time.Minute {
		t.Fatalf("loaded config = %#v", value)
	}
	username, password, err := value.ECS.Clusters[0].Credentials()
	if err != nil || username != "monitor" || password != "secret-password" {
		t.Fatalf("credentials = %q %q err=%v", username, password, err)
	}
}

func TestLoadAndValidationFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unknown field", content: "unknown: true\n", want: "field unknown"},
		{name: "missing environment", content: "server:\n  listenAddress: ${MISSING_TEST_VALUE}\n", want: "unset environment"},
		{name: "no clusters", content: "{}\n", want: "at least one cluster"},
		{
			name: "duplicate clusters",
			content: `
ecs:
  clusters:
    - name: same
      environment: test
      endpoint: https://ecs.example.invalid
      username: monitor
      password: test-password
      tls:
        verify: false
    - name: same
      environment: test
      endpoint: https://ecs.example.invalid
      username: monitor
      password: test-password
      tls:
        verify: false
`,
			want: "duplicated",
		},
		{
			name:    "bad duration",
			content: "cache:\n  staleTolerance: never\n",
			want:    "time: invalid duration",
		},
		{
			name:    "endpoint without host",
			content: strings.Replace(validClusterYAML("test", "test", "false"), "https://ecs.example.invalid", "https://:4443", 1),
			want:    "origin-only HTTPS URL",
		},
		{
			name:    "unsafe cluster name",
			content: validClusterYAML("' bad '", "test", "false"),
			want:    "trimmed, printable",
		},
		{
			name: "basic auth colon",
			content: `
ecs:
  clusters:
    - name: test
      site: lab
      environment: test
      endpoint: https://ecs.example.invalid
      username: 'bad:name'
      password: test-password
      tls:
        verify: false
`,
			want: "cannot contain a colon",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestProductionClusterMayExplicitlyDisableTLSVerification(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(
		path, []byte("security:\n  inventoryApi:\n    enabled: false\n"+
			validClusterYAML("prod", "production", "false")), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ECS.Clusters[0].TLS.VerificationEnabled() {
		t.Fatal("explicit production tls.verify=false was not preserved")
	}
}

func TestInventorySecurityValidation(t *testing.T) {
	t.Parallel()
	value := Defaults()
	value.ECS.Clusters = []ClusterConfig{testCluster()}
	value.Security.InventoryAPI.Authentication = "proxy"
	value.Security.InventoryAPI.TrustedProxyCIDRs = []string{"bad"}
	if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "trusted proxy CIDR") {
		t.Fatalf("Validate error = %v", err)
	}
	value.Security.InventoryAPI.TrustedProxyCIDRs = []string{"192.0.2.0/24"}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestIntervalOverrides(t *testing.T) {
	t.Parallel()
	defaults := Defaults().Collector.Intervals
	override := Duration{Duration: 42 * time.Second}
	values := (IntervalOverride{Bucket: &override}).Resolve(defaults)
	if values["bucket"] != 42*time.Second || values["cluster"] != time.Minute {
		t.Fatalf("resolved intervals = %v", values)
	}
}

func TestTimeoutOverrides(t *testing.T) {
	t.Parallel()
	connect := Duration{Duration: 2 * time.Second}
	overall := Duration{Duration: 20 * time.Second}
	read := Duration{Duration: 10 * time.Second}
	connectValue, readValue, overallValue := (TimeoutOverride{
		Connect: &connect, Read: &read, Overall: &overall,
	}).Resolve(30 * time.Second)
	if connectValue != 2*time.Second || readValue != 10*time.Second ||
		overallValue != 20*time.Second {
		t.Fatalf("timeouts = %s %s %s", connectValue, readValue, overallValue)
	}
	value := Defaults()
	value.Security.InventoryAPI.Enabled = false
	value.ECS.Clusters = []ClusterConfig{testCluster()}
	tooLong := Duration{Duration: time.Minute}
	value.ECS.Clusters[0].Timeouts.Read = &tooLong
	if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "timeouts") {
		t.Fatalf("invalid timeout error = %v", err)
	}
}

func TestReplicationIDCannotCrossKinds(t *testing.T) {
	t.Parallel()
	value := Defaults()
	value.Security.InventoryAPI.Enabled = false
	cluster := testCluster()
	cluster.Replication.Groups = []string{"same"}
	cluster.Replication.Links = []string{"same"}
	value.ECS.Clusters = []ClusterConfig{cluster}
	if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "across groups and links") {
		t.Fatalf("duplicate cross-kind replication ID error = %v", err)
	}
}

func TestReservedMetricsRoot(t *testing.T) {
	t.Parallel()
	value := Defaults()
	value.Prometheus.Path = "/api/v1"
	value.Security.InventoryAPI.Enabled = false
	value.ECS.Clusters = []ClusterConfig{testCluster()}
	if err := value.Validate(); err == nil || !strings.Contains(err.Error(), "reserved API path") {
		t.Fatalf("reserved metrics path error = %v", err)
	}
}

func TestSecretFilePreservesPasswordWhitespace(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte(" secret with spaces \r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := resolveSecret("", path)
	if err != nil || value != " secret with spaces " {
		t.Fatalf("resolved password = %q err=%v", value, err)
	}
}

func TestRepositoryExampleConfiguration(t *testing.T) {
	t.Parallel()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	content, err := os.ReadFile(filepath.Join(root, "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for name, value := range map[string]string{
		"username":        "monitor\n",
		"password":        "test password\n",
		"inventory-token": "0123456789abcdef-token\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved := strings.ReplaceAll(
		string(content),
		".local-secrets/",
		filepath.ToSlash(directory)+"/",
	)
	configPath := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(configPath, []byte(resolved), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("repository example is invalid: %v", err)
	}
	if len(loaded.ECS.Clusters) != 1 || loaded.ECS.Clusters[0].Name != "primary-ecs" {
		t.Fatalf("repository example loaded unexpected clusters: %#v", loaded.ECS.Clusters)
	}
}

func TestRateLimitConditionalCapabilityAndNodeResourceValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "zero rate",
			edit: func(value *Config) { value.Collector.RateLimit.RequestsPerSecond = 0 },
			want: "requestsPerSecond",
		},
		{
			name: "zero burst",
			edit: func(value *Config) { value.Collector.RateLimit.Burst = 0 },
			want: "rateLimit.burst",
		},
		{
			name: "unknown conditional capability",
			edit: func(value *Config) {
				value.ECS.Clusters[0].Capabilities.EnabledConditional = []string{"not_real"}
			},
			want: "is unsupported",
		},
		{
			name: "disk without filesystem allowlist",
			edit: func(value *Config) {
				value.ECS.Clusters[0].Capabilities.EnabledConditional =
					[]string{"node_disk_capacity"}
			},
			want: "requires nodeResources.filesystems",
		},
		{
			name: "duplicate network interface",
			edit: func(value *Config) {
				value.ECS.Clusters[0].NodeResources.NetworkInterfaces = []string{"bond0", "bond0"}
			},
			want: "is duplicated",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := Defaults()
			value.Security.InventoryAPI.Enabled = false
			value.ECS.Clusters = []ClusterConfig{testCluster()}
			test.edit(&value)
			if err := value.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, test.want)
			}
		})
	}

	value := Defaults()
	value.Security.InventoryAPI.Enabled = false
	cluster := testCluster()
	cluster.Capabilities.EnabledConditional = []string{"node_disk_capacity"}
	cluster.NodeResources.Filesystems = []string{"/data"}
	value.ECS.Clusters = []ClusterConfig{cluster}
	if err := value.Validate(); err != nil {
		t.Fatalf("valid conditional node disk configuration: %v", err)
	}
}

func validClusterYAML(name, environment, verify string) string {
	return `
ecs:
  clusters:
    - name: ` + name + `
      site: lab
      environment: ` + environment + `
      endpoint: https://ecs.example.invalid
      username: monitor
      password: test-password
      tls:
        verify: ` + verify + `
`
}

func testCluster() ClusterConfig {
	verify := false
	return ClusterConfig{
		Name: "test", Site: "lab", Environment: "test", Endpoint: "https://ecs.example.invalid",
		Username: "monitor", Password: "test-password", TLS: ClusterTLSConfig{Verify: &verify},
	}
}
