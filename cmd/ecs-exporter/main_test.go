package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunValidationModes(t *testing.T) {
	t.Parallel()
	profiles := filepath.Join(repositoryRoot(t), "profiles")
	stdout := createOutputFile(t, "stdout")
	stderr := createOutputFile(t, "stderr")

	if err := run(
		[]string{"-profiles-dir", profiles, "-validate-profiles"},
		stdout, stderr,
	); err != nil {
		t.Fatal(err)
	}
	var profileResult struct {
		Status   string `json:"status"`
		Profiles string `json:"profiles"`
	}
	decodeOutput(t, stdout, &profileResult)
	if profileResult.Status != "valid" ||
		profileResult.Profiles != "ecs-3.6,ecs-3.7,ecs-3.8.0,ecs-3.8.1" {
		t.Fatalf("profile validation result = %#v", profileResult)
	}

	directory := t.TempDir()
	usernameFile := filepath.Join(directory, "username")
	passwordFile := filepath.Join(directory, "password")
	tokenFile := filepath.Join(directory, "inventory-token")
	for path, content := range map[string]string{
		usernameFile: "monitor\n",
		passwordFile: "test password\n",
		tokenFile:    "0123456789abcdef-token\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(directory, "config.yaml")
	configContent := "security:\n" +
		"  inventoryApi:\n" +
		"    tokenFile: " + tokenFile + "\n" +
		"ecs:\n" +
		"  clusters:\n" +
		"    - name: test\n" +
		"      site: lab\n" +
		"      environment: test\n" +
		"      endpoint: https://ecs.example.invalid\n" +
		"      usernameFile: " + usernameFile + "\n" +
		"      passwordFile: " + passwordFile + "\n" +
		"      tls:\n" +
		"        verify: false\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout = createOutputFile(t, "config-stdout")
	if err := run(
		[]string{
			"-profiles-dir", profiles, "-config", configPath,
			"-validate-config",
		},
		stdout, stderr,
	); err != nil {
		t.Fatal(err)
	}
	var configResult struct {
		Status   string `json:"status"`
		Clusters int    `json:"clusters"`
	}
	decodeOutput(t, stdout, &configResult)
	if configResult.Status != "valid" || configResult.Clusters != 1 {
		t.Fatalf("config validation result = %#v", configResult)
	}

	if err := run([]string{"unexpected"}, stdout, stderr); err == nil {
		t.Fatal("positional argument was accepted")
	}
	if err := run([]string{"-help"}, stdout, stderr); err != nil {
		t.Fatalf("help returned an error: %v", err)
	}
}

func TestAccessLogOmitsQueryAndRecordsStatus(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := accessLog(logger, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/health?token=sensitive", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	logged := output.String()
	if recorder.Code != http.StatusNoContent || !strings.Contains(logged, `"status":204`) ||
		!strings.Contains(logged, `"path":"/health"`) ||
		strings.Contains(logged, "sensitive") {
		t.Fatalf("access log = %s", logged)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("ECS_EXPORTER_TEST_VALUE", "configured")
	if envOrDefault("ECS_EXPORTER_TEST_VALUE", "fallback") != "configured" ||
		envOrDefault("ECS_EXPORTER_TEST_MISSING", "fallback") != "fallback" {
		t.Fatal("environment fallback is incorrect")
	}
}

func createOutputFile(t *testing.T, name string) *os.File {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func decodeOutput(t *testing.T, file *os.File, target any) {
	t.Helper()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(file).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
