package profile

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fixtureManifest struct {
	SchemaVersion   string `json:"schema_version"`
	ProfileID       string `json:"profile_id"`
	Inherits        string `json:"inherits"`
	Classification  string `json:"classification"`
	SandboxEvidence bool   `json:"sandbox_evidence"`
	Fixtures        []struct {
		File      string `json:"file"`
		MappingID string `json:"mapping_id"`
		Scenario  string `json:"scenario"`
		Evidence  string `json:"evidence"`
	} `json:"fixtures"`
}

type fluxResponse struct {
	Series []struct {
		Datatypes []string   `json:"Datatypes"`
		Columns   []string   `json:"Columns"`
		Values    [][]string `json:"Values"`
	} `json:"Series"`
}

func TestRepositoryFixtureContracts(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	fixtureRoot := filepath.Join(root, "testdata", "ecs")
	mappingText := readMappingDocuments(t, filepath.Join(root, "docs", "ecs-api"))
	credentialPattern := regexp.MustCompile(
		`(?i)(authorization|cookie|x-sds-auth-token)"?\s*[:=]\s*"?(basic|bearer)?\s*[A-Za-z0-9._~+/=-]{12,}`,
	)

	jsonCount := 0
	manifestCount := 0
	fixtureCount := 0
	fluxCount := 0
	err := filepath.WalkDir(fixtureRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		jsonCount++
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var parsed any
		if decodeErr := json.Unmarshal(content, &parsed); decodeErr != nil {
			t.Errorf("%s is invalid JSON: %v", path, decodeErr)
		}
		privateKeyMarker := "-----BEGIN " + "PRIVATE KEY-----"
		if credentialPattern.Match(content) || strings.Contains(string(content), privateKeyMarker) {
			t.Errorf("%s contains credential-like material", path)
		}

		switch entry.Name() {
		case "manifest.json":
			manifestCount++
			fixtureCount += validateFixtureManifest(t, path, content, mappingText)
		default:
			if strings.HasPrefix(entry.Name(), "flux-") {
				fluxCount++
				validateFluxFixture(t, path, content)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if jsonCount != 45 {
		t.Errorf("fixture JSON count = %d, want 45", jsonCount)
	}
	if manifestCount != 7 || fixtureCount != 37 {
		t.Errorf("manifest count = %d fixture records = %d, want 7 and 37", manifestCount, fixtureCount)
	}
	if fluxCount != 6 {
		t.Errorf("Flux fixture count = %d, want 6", fluxCount)
	}
}

func validateFixtureManifest(t *testing.T, path string, content, mappingText []byte) int {
	t.Helper()
	var manifest fixtureManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Errorf("%s cannot decode manifest: %v", path, err)
		return 0
	}
	validClassification := manifest.Classification == fixtureClassSynthetic ||
		manifest.Classification == fixtureClassSandbox
	if manifest.SchemaVersion != SchemaVersion || !validClassification ||
		manifest.SandboxEvidence != (manifest.Classification == fixtureClassSandbox) {
		t.Errorf("%s has invalid evidence classification", path)
	}
	if manifest.Inherits != "" {
		inherited := filepath.Clean(filepath.Join(filepath.Dir(path), manifest.Inherits))
		if _, err := os.Stat(inherited); err != nil {
			t.Errorf("%s inherits missing manifest %s: %v", path, inherited, err)
		}
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.File == "" || fixture.MappingID == "" || fixture.Scenario == "" ||
			fixture.Evidence != manifest.Classification {
			t.Errorf("%s contains incomplete fixture record %#v", path, fixture)
			continue
		}
		fixturePath := filepath.Clean(filepath.Join(filepath.Dir(path), fixture.File))
		if filepath.Dir(fixturePath) != filepath.Dir(path) {
			t.Errorf("%s fixture escapes its manifest directory: %s", path, fixture.File)
		}
		if _, err := os.Stat(fixturePath); err != nil {
			t.Errorf("%s references missing fixture %s: %v", path, fixture.File, err)
		}
		if !strings.Contains(string(mappingText), fixture.MappingID) {
			t.Errorf("%s references undocumented mapping %s", path, fixture.MappingID)
		}
	}
	return len(manifest.Fixtures)
}

func validateFluxFixture(t *testing.T, path string, content []byte) {
	t.Helper()
	var response fluxResponse
	if err := json.Unmarshal(content, &response); err != nil {
		t.Errorf("%s cannot decode Flux response: %v", path, err)
		return
	}
	if len(response.Series) == 0 {
		t.Errorf("%s has no Flux series", path)
		return
	}

	outOfWindow := 0
	for seriesIndex, series := range response.Series {
		if len(series.Columns) != len(series.Datatypes) {
			t.Errorf("%s series %d column/type length mismatch", path, seriesIndex)
		}
		index := make(map[string]int, len(series.Columns))
		for columnIndex, column := range series.Columns {
			index[column] = columnIndex
		}
		startIndex, hasStart := index["_start"]
		stopIndex, hasStop := index["_stop"]
		timeIndex, hasTime := index["_time"]
		if !hasTime {
			t.Errorf("%s series %d lacks the Flux _time column", path, seriesIndex)
			continue
		}
		for rowIndex, row := range series.Values {
			if len(row) != len(series.Columns) {
				t.Errorf("%s series %d row %d length mismatch", path, seriesIndex, rowIndex)
				continue
			}
			sample := parseFixtureTime(t, path, row[timeIndex])
			if !hasStart || !hasStop {
				continue
			}
			start := parseFixtureTime(t, path, row[startIndex])
			stop := parseFixtureTime(t, path, row[stopIndex])
			if sample.Before(start) || !sample.Before(stop) {
				outOfWindow++
			}
		}
	}

	isDefectFixture := strings.Contains(filepath.Base(path), "range-out-of-window")
	if isDefectFixture && outOfWindow == 0 {
		t.Errorf("%s should contain an out-of-window sample", path)
	}
	if !isDefectFixture && outOfWindow != 0 {
		t.Errorf("%s unexpectedly contains %d out-of-window samples", path, outOfWindow)
	}
}

func parseFixtureTime(t *testing.T, path, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Errorf("%s contains invalid RFC3339 time %q: %v", path, value, err)
	}
	return parsed
}

func readMappingDocuments(t *testing.T, dir string) []byte {
	t.Helper()
	var combined strings.Builder
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(content)
	}
	return []byte(combined.String())
}
