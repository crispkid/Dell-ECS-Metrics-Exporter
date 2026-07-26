package main

import (
	"strings"
	"testing"
)

func TestValidateMetrics(t *testing.T) {
	input := strings.NewReader(`# HELP required_metric test metric
# TYPE required_metric gauge
required_metric 1
`)
	count, err := validateMetrics(input, []string{"required_metric"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("metric family count = %d, want 1", count)
	}
}

func TestValidateMetricsRejectsMissingFamily(t *testing.T) {
	_, err := validateMetrics(
		strings.NewReader("present_metric 1\n"),
		[]string{"missing_metric"},
		nil,
	)
	if err == nil {
		t.Fatal("missing metric family was accepted")
	}
}

func TestValidateMetricsRejectsMalformedExposition(t *testing.T) {
	_, err := validateMetrics(strings.NewReader("not valid metrics\n"), nil, nil)
	if err == nil {
		t.Fatal("malformed exposition was accepted")
	}
}

func TestValidateMetricsCollectorErrorAllowlist(t *testing.T) {
	input := `# TYPE ecs_exporter_collector_errors_total counter
ecs_exporter_collector_errors_total{cluster="alpha",collector="node-resources"} 1
`
	if _, err := validateMetrics(
		strings.NewReader(input), nil, []string{"node-resources"},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := validateMetrics(
		strings.NewReader(input), nil, []string{"bucket"},
	); err == nil {
		t.Fatal("unexpected collector error was accepted")
	}
}
