package main

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type ruleFile struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string `yaml:"name"`
	Rules []rule `yaml:"rules"`
}

type rule struct {
	Alert       string            `yaml:"alert"`
	Expression  string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: rulescheck PATH")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		fail(err)
	}
	defer file.Close()

	var value ruleFile
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&value); err != nil {
		fail(fmt.Errorf("decode rules: %w", err))
	}
	if len(value.Groups) == 0 {
		fail(fmt.Errorf("at least one rule group is required"))
	}
	seen := make(map[string]struct{})
	for _, group := range value.Groups {
		if group.Name == "" || len(group.Rules) == 0 {
			fail(fmt.Errorf("every group needs a name and at least one rule"))
		}
		for _, item := range group.Rules {
			if item.Alert == "" || item.Expression == "" || item.For == "" {
				fail(fmt.Errorf("group %s has an incomplete alert", group.Name))
			}
			if _, exists := seen[item.Alert]; exists {
				fail(fmt.Errorf("duplicate alert name %s", item.Alert))
			}
			seen[item.Alert] = struct{}{}
			if item.Labels["severity"] == "" ||
				item.Annotations["summary"] == "" ||
				item.Annotations["description"] == "" ||
				item.Annotations["runbook"] == "" {
				fail(fmt.Errorf("alert %s is missing operator metadata", item.Alert))
			}
		}
	}
	fmt.Printf("valid alert rules: %d\n", len(seen))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
