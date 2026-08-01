package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/prometheus/common/expfmt"
	prommodel "github.com/prometheus/common/model"
)

type stringList []string

func (values *stringList) String() string {
	return fmt.Sprint(*values)
}

func (values *stringList) Set(value string) error {
	if value == "" {
		return fmt.Errorf("metric family name cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	required := stringList{
		"ecs_exporter_build_info",
		"ecs_exporter_profile_contract_info",
		"ecs_exporter_cached_resources",
	}
	flag.Var(&required, "require", "additional metric family that must be present (repeatable)")
	allowedCollectorErrors := stringList{}
	flag.Var(
		&allowedCollectorErrors,
		"allow-collector-error",
		"collector allowed to have a positive error counter (repeatable)",
	)
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "metricscheck accepts Prometheus exposition on stdin only")
		os.Exit(2)
	}

	count, err := validateMetrics(os.Stdin, required, allowedCollectorErrors)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("valid metric families: %d\n", count)
}

func validateMetrics(input io.Reader, required, allowedCollectorErrors []string) (int, error) {
	parser := expfmt.NewTextParser(prommodel.LegacyValidation)
	families, err := parser.TextToMetricFamilies(input)
	if err != nil {
		return 0, fmt.Errorf("invalid Prometheus exposition: %w", err)
	}
	for _, name := range required {
		if _, ok := families[name]; !ok {
			return 0, fmt.Errorf("required metric family is missing: %s", name)
		}
	}
	if len(allowedCollectorErrors) > 0 {
		family := families["ecs_exporter_collector_errors_total"]
		if family != nil {
			for _, metric := range family.Metric {
				counter := metric.GetCounter()
				if counter == nil {
					return 0, fmt.Errorf(
						"ecs_exporter_collector_errors_total is not a counter",
					)
				}
				if counter.GetValue() <= 0 {
					continue
				}
				collector := ""
				for _, pair := range metric.Label {
					if pair.GetName() == "collector" {
						collector = pair.GetValue()
						break
					}
				}
				if !slices.Contains(allowedCollectorErrors, collector) {
					return 0, fmt.Errorf(
						"collector %q has a positive error counter and is not allowed",
						collector,
					)
				}
			}
		}
	}
	return len(families), nil
}
