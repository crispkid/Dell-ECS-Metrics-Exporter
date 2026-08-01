package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dell-ecs-metrics-exporter/internal/fluxprobe"
)

func TestRunHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "-performance") || stdout.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunEmitsRedactedSetupFailure(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "private-cluster-name.yaml")
	var stdout, stderr bytes.Buffer
	err := run([]string{"-config", missing}, &stdout, &stderr)
	if !errors.Is(err, errProbeDidNotPass) {
		t.Fatalf("error = %v", err)
	}
	var report fluxprobe.Report
	if decodeErr := json.Unmarshal(stdout.Bytes(), &report); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if report.Result != fluxprobe.ResultFailed || report.Checks[0].ErrorType != "configuration" {
		t.Fatalf("report = %#v", report)
	}
	if strings.Contains(stdout.String(), missing) || strings.Contains(stdout.String(), "private-cluster") ||
		stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
