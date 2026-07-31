package profile

import (
	"fmt"
	"slices"
	"strings"
)

// Catalog is an immutable set of validated, non-overlapping profiles.
type Catalog struct {
	profiles []*Profile
}

// Resolution is the conservative capability contract for one ECS cluster.
type Resolution struct {
	ProfileIDs   []string
	Mixed        bool
	Capabilities map[string]Support
}

// LoadDir loads and validates all ecs-*.json files in a directory.
func LoadDir(dir string) (*Catalog, error) {
	paths, err := profileFiles(dir)
	if err != nil {
		return nil, err
	}

	profiles := make([]*Profile, 0, len(paths))
	ids := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		value, err := loadProfile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, exists := ids[value.ProfileID]; exists {
			return nil, fmt.Errorf("duplicate profile_id %q", value.ProfileID)
		}
		ids[value.ProfileID] = struct{}{}
		profiles = append(profiles, value)
	}

	slices.SortFunc(profiles, func(left, right *Profile) int {
		return left.minVersion.Compare(right.minVersion)
	})
	for i := 1; i < len(profiles); i++ {
		if profiles[i].minVersion.Compare(profiles[i-1].maxVersion) < 0 {
			return nil, fmt.Errorf(
				"profile ranges overlap: %s and %s",
				profiles[i-1].ProfileID,
				profiles[i].ProfileID,
			)
		}
	}
	return &Catalog{profiles: profiles}, nil
}

// Profiles returns independent copies in catalog order.
func (c *Catalog) Profiles() []*Profile {
	profiles := make([]*Profile, 0, len(c.profiles))
	for _, value := range c.profiles {
		profiles = append(profiles, cloneProfile(value))
	}
	return profiles
}

// Match returns the unique profile for a version.
func (c *Catalog) Match(version Version) (*Profile, error) {
	for _, candidate := range c.profiles {
		if candidate.Supports(version) {
			return cloneProfile(candidate), nil
		}
	}
	return nil, fmt.Errorf("unsupported ECS version %q", version.String())
}

// Resolve selects profiles for every node version. During a rolling upgrade it
// returns only conservative shared capabilities and always disables
// interval-derived Flux rates.
func (c *Catalog) Resolve(nodeVersions []string) (Resolution, error) {
	if len(nodeVersions) == 0 {
		return Resolution{}, fmt.Errorf("node version list is empty")
	}

	var selected []*Profile
	seen := make(map[string]struct{})
	for index, raw := range nodeVersions {
		version, err := ParseVersion(raw)
		if err != nil {
			return Resolution{}, fmt.Errorf("node version %d: %w", index, err)
		}
		matched, err := c.Match(version)
		if err != nil {
			return Resolution{}, fmt.Errorf("node version %d: %w", index, err)
		}
		if _, exists := seen[matched.ProfileID]; exists {
			continue
		}
		seen[matched.ProfileID] = struct{}{}
		selected = append(selected, matched)
	}

	slices.SortFunc(selected, func(left, right *Profile) int {
		return left.minVersion.Compare(right.minVersion)
	})
	capabilities := cloneCapabilities(selected[0].Capabilities)
	for _, current := range selected[1:] {
		for name, support := range capabilities {
			other, exists := current.Capabilities[name]
			if !exists {
				capabilities[name] = SupportUnavailable
				continue
			}
			capabilities[name] = intersectSupport(support, other)
		}
	}

	mixed := len(selected) > 1
	if mixed {
		capabilities["flux_interval_rates"] = SupportUnavailable
	}
	ids := make([]string, 0, len(selected))
	for _, value := range selected {
		ids = append(ids, value.ProfileID)
	}
	return Resolution{ProfileIDs: ids, Mixed: mixed, Capabilities: capabilities}, nil
}

func cloneCapabilities(source map[string]Support) map[string]Support {
	result := make(map[string]Support, len(source))
	for name, support := range source {
		result[name] = support
	}
	return result
}

func cloneProfile(source *Profile) *Profile {
	result := *source
	result.Version.DocumentedRelease = slices.Clone(source.Version.DocumentedRelease)
	result.Version.TestedBuilds = slices.Clone(source.Version.TestedBuilds)
	result.Capabilities = cloneCapabilities(source.Capabilities)
	result.KnownIssues = slices.Clone(source.KnownIssues)
	result.Evidence.OfficialSources = slices.Clone(source.Evidence.OfficialSources)
	result.Evidence.SharedValidatedCapabilities = slices.Clone(
		source.Evidence.SharedValidatedCapabilities,
	)
	return &result
}

func intersectSupport(left, right Support) Support {
	if left == right {
		return left
	}
	if left == SupportUnavailable || right == SupportUnavailable {
		return SupportUnavailable
	}
	return SupportConditional
}

// Summary returns a compact human-readable catalog description.
func (c *Catalog) Summary() string {
	ids := make([]string, 0, len(c.profiles))
	for _, value := range c.profiles {
		ids = append(ids, value.ProfileID)
	}
	return strings.Join(ids, ",")
}
