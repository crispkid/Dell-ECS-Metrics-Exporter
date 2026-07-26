package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type nodeResponse struct {
	Nodes []struct {
		Version string `json:"version"`
	} `json:"node"`
}

func TestLoadRepositoryProfiles(t *testing.T) {
	t.Parallel()
	catalog := loadRepositoryCatalog(t)
	profiles := catalog.Profiles()
	if len(profiles) != 4 {
		t.Fatalf("profile count = %d, want 4", len(profiles))
	}
	if got := catalog.Summary(); got != "ecs-3.6,ecs-3.7,ecs-3.8.0,ecs-3.8.1" {
		t.Fatalf("summary = %q", got)
	}
	for _, value := range profiles {
		if value.Evidence.SandboxCertified {
			t.Errorf("%s unexpectedly sandbox certified", value.ProfileID)
		}
		if len(value.Version.TestedBuilds) != 0 {
			t.Errorf("%s tested builds = %v, want empty", value.ProfileID, value.Version.TestedBuilds)
		}
		if !value.Supports(mustVersion(t, value.Version.MinInclusive)) {
			t.Errorf("%s does not support inclusive lower bound", value.ProfileID)
		}
		if value.Supports(mustVersion(t, value.Version.MaxExclusive)) {
			t.Errorf("%s supports exclusive upper bound", value.ProfileID)
		}
	}

	copyOfProfiles := catalog.Profiles()
	copyOfProfiles[0].ProfileID = "mutated"
	copyOfProfiles[0].Capabilities["authentication"] = SupportUnavailable
	freshProfile := catalog.Profiles()[0]
	if freshProfile.ProfileID != "ecs-3.6" ||
		freshProfile.Capabilities["authentication"] != SupportNative {
		t.Fatal("Profiles returned mutable catalog data")
	}
}

func TestResolveVersionFixtures(t *testing.T) {
	t.Parallel()
	catalog := loadRepositoryCatalog(t)
	cases := []struct {
		dir       string
		profileID string
	}{
		{dir: "ecs-3.6", profileID: "ecs-3.6"},
		{dir: "ecs-3.7", profileID: "ecs-3.7"},
		{dir: "ecs-3.8.0", profileID: "ecs-3.8.0"},
		{dir: "ecs-3.8.1", profileID: "ecs-3.8.1"},
	}
	for _, test := range cases {
		test := test
		t.Run(test.profileID, func(t *testing.T) {
			t.Parallel()
			versions := readNodeVersions(t, filepath.Join(repositoryRoot(t), "testdata", "ecs", test.dir, "nodes.json"))
			resolution, err := catalog.Resolve(versions)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.Mixed || !slices.Equal(resolution.ProfileIDs, []string{test.profileID}) {
				t.Fatalf("resolution = %#v", resolution)
			}
			if resolution.Capabilities["authentication"] != SupportNative {
				t.Errorf("authentication = %q, want native", resolution.Capabilities["authentication"])
			}
			resolution.Capabilities["authentication"] = SupportUnavailable
			second, err := catalog.Resolve(versions)
			if err != nil {
				t.Fatal(err)
			}
			if second.Capabilities["authentication"] != SupportNative {
				t.Fatal("resolution returned mutable catalog capability map")
			}
		})
	}
}

func TestResolveMixedAndUnsupportedFixtures(t *testing.T) {
	t.Parallel()
	catalog := loadRepositoryCatalog(t)
	commonDir := filepath.Join(repositoryRoot(t), "testdata", "ecs", "common")

	mixed, err := catalog.Resolve(readNodeVersions(t, filepath.Join(commonDir, "nodes-mixed-version.json")))
	if err != nil {
		t.Fatal(err)
	}
	if !mixed.Mixed {
		t.Fatal("mixed fixture was not detected")
	}
	if want := []string{"ecs-3.7", "ecs-3.8.0"}; !slices.Equal(mixed.ProfileIDs, want) {
		t.Fatalf("profile IDs = %v, want %v", mixed.ProfileIDs, want)
	}
	if mixed.Capabilities["flux_interval_rates"] != SupportUnavailable {
		t.Fatalf("mixed interval rates = %q, want unavailable", mixed.Capabilities["flux_interval_rates"])
	}
	if mixed.Capabilities["vdc_performance"] != SupportConditional {
		t.Fatalf("mixed VDC performance = %q, want conditional", mixed.Capabilities["vdc_performance"])
	}

	_, err = catalog.Resolve(readNodeVersions(t, filepath.Join(commonDir, "nodes-unsupported-version.json")))
	if err == nil || !strings.Contains(err.Error(), `unsupported ECS version "3.8.2.0.123456.synthetic"`) {
		t.Fatalf("unsupported resolution error = %v", err)
	}
}

func TestResolveRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	catalog := loadRepositoryCatalog(t)
	if _, err := catalog.Resolve(nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty versions error = %v", err)
	}
	if _, err := catalog.Resolve([]string{"3.8.x.1"}); err == nil || !strings.Contains(err.Error(), "node version 0") {
		t.Fatalf("invalid versions error = %v", err)
	}
}

func TestLoadDirRejectsInvalidCatalogs(t *testing.T) {
	t.Parallel()
	validPath := filepath.Join(repositoryRoot(t), "profiles", "ecs-3.6.json")
	valid, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		files       map[string]string
		wantErrPart string
	}{
		{name: "empty", files: map[string]string{}, wantErrPart: "no ecs-*.json"},
		{
			name: "unknown field",
			files: map[string]string{
				"ecs-bad.json": strings.Replace(string(valid), `"schema_version": "1.0",`, `"schema_version": "1.0", "unexpected": true,`, 1),
			},
			wantErrPart: "unknown field",
		},
		{
			name: "trailing value",
			files: map[string]string{
				"ecs-bad.json": string(valid) + "\n{}",
			},
			wantErrPart: "multiple JSON values",
		},
		{
			name: "invalid JSON",
			files: map[string]string{
				"ecs-bad.json": "{",
			},
			wantErrPart: "unexpected EOF",
		},
		{
			name: "overlap",
			files: map[string]string{
				"ecs-first.json":  string(valid),
				"ecs-second.json": strings.Replace(string(valid), `"profile_id": "ecs-3.6"`, `"profile_id": "ecs-3.7"`, 1),
			},
			wantErrPart: "overlap",
		},
		{
			name: "unsupported profile ID",
			files: map[string]string{
				"ecs-bad.json": strings.Replace(string(valid), `"profile_id": "ecs-3.6"`, `"profile_id": "ecs-copy"`, 1),
			},
			wantErrPart: "profile_id",
		},
		{
			name: "unsubstantiated certification",
			files: map[string]string{
				"ecs-bad.json": strings.Replace(
					strings.Replace(string(valid), `"tested_builds": []`, `"tested_builds": ["3.6.2.6"]`, 1),
					`"sandbox_certified": false`,
					`"sandbox_certified": true`,
					1,
				),
			},
			wantErrPart: "sandbox_certified",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for name, content := range test.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := LoadDir(dir)
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("LoadDir error = %v, want containing %q", err, test.wantErrPart)
			}
		})
	}
}

func TestLoadDirAcceptsCompleteSandboxCertificationEvidence(t *testing.T) {
	t.Parallel()
	validPath := filepath.Join(repositoryRoot(t), "profiles", "ecs-3.6.json")
	content, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatal(err)
	}
	certified := strings.Replace(string(content), `"tested_builds": []`,
		`"tested_builds": ["3.6.2.6"]`, 1)
	certified = strings.Replace(certified, `"status": "documentation-verified"`,
		`"status": "sandbox-verified"`, 1)
	certified = strings.Replace(certified,
		`"fixture_classification": "synthetic-document-derived"`,
		`"fixture_classification": "redacted-sandbox-derived"`, 1)
	certified = strings.Replace(certified, `"sandbox_certified": false`,
		`"sandbox_certified": true`, 1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ecs-3.6.json"), []byte(certified), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Profiles()[0].Evidence.SandboxCertified {
		t.Fatal("complete sandbox certification evidence was not preserved")
	}
}

func TestIntersectSupport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		left, right, want Support
	}{
		{SupportNative, SupportNative, SupportNative},
		{SupportNative, SupportUnavailable, SupportUnavailable},
		{SupportUnavailable, SupportDerived, SupportUnavailable},
		{SupportNative, SupportDerived, SupportConditional},
	}
	for _, test := range tests {
		if got := intersectSupport(test.left, test.right); got != test.want {
			t.Errorf("intersectSupport(%q, %q) = %q, want %q", test.left, test.right, got, test.want)
		}
	}
}

func loadRepositoryCatalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := LoadDir(filepath.Join(repositoryRoot(t), "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readNodeVersions(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var response nodeResponse
	if err := json.NewDecoder(file).Decode(&response); err != nil {
		t.Fatal(err)
	}
	versions := make([]string, 0, len(response.Nodes))
	for _, node := range response.Nodes {
		versions = append(versions, node.Version)
	}
	return versions
}
