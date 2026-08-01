package profile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(
	`^([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)([._+-][0-9A-Za-z][0-9A-Za-z._+-]*)?$`,
)

// Version is a Dell ECS four-part product version. A build suffix is retained
// for diagnostics but does not participate in profile range comparisons.
type Version struct {
	parts  [4]uint32
	suffix string
	raw    string
}

// ParseVersion parses a Dell ECS version such as 3.8.1.7 or
// 3.8.1.7.123456.synthetic.
func ParseVersion(value string) (Version, error) {
	if value == "" {
		return Version{}, fmt.Errorf("version is empty")
	}
	if value != strings.TrimSpace(value) {
		return Version{}, fmt.Errorf("version %q contains surrounding whitespace", value)
	}

	matches := versionPattern.FindStringSubmatch(value)
	if matches == nil {
		return Version{}, fmt.Errorf(
			"version %q must contain four numeric parts and an optional build suffix",
			value,
		)
	}

	var parsed Version
	parsed.raw = value
	parsed.suffix = matches[5]
	for i := range parsed.parts {
		part, err := strconv.ParseUint(matches[i+1], 10, 32)
		if err != nil {
			return Version{}, fmt.Errorf("version %q part %d: %w", value, i+1, err)
		}
		parsed.parts[i] = uint32(part)
	}

	return parsed, nil
}

// Compare returns -1, 0, or 1 after comparing the four product-version parts.
func (v Version) Compare(other Version) int {
	for i := range v.parts {
		switch {
		case v.parts[i] < other.parts[i]:
			return -1
		case v.parts[i] > other.parts[i]:
			return 1
		}
	}
	return 0
}

// Core returns the normalized four-part product version.
func (v Version) Core() string {
	return fmt.Sprintf("%d.%d.%d.%d", v.parts[0], v.parts[1], v.parts[2], v.parts[3])
}

// Suffix returns the optional build suffix, including its first separator.
func (v Version) Suffix() string {
	return v.suffix
}

func (v Version) String() string {
	return v.raw
}
