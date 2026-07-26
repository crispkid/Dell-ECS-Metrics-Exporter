package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	SchemaVersion = "1.0"

	versionParser         = "dell-four-part-with-optional-build-suffix"
	fixtureClassSynthetic = "synthetic-document-derived"
	fixtureClassSandbox   = "redacted-sandbox-derived"
	versionDiscoveryAPI   = "version-discovery"
)

var validProfileIDs = []string{"ecs-3.6", "ecs-3.7", "ecs-3.8.0", "ecs-3.8.1"}

// Support describes how a logical ECS capability can be provided.
type Support string

const (
	SupportNative      Support = "native"
	SupportDerived     Support = "derived"
	SupportConditional Support = "conditional"
	SupportUnavailable Support = "unavailable"
)

// Profile is the machine-readable compatibility contract for one ECS range.
type Profile struct {
	SchemaVersion string             `json:"schema_version"`
	ProfileID     string             `json:"profile_id"`
	Version       VersionContract    `json:"version"`
	Selection     SelectionContract  `json:"selection"`
	Transport     TransportContract  `json:"transport"`
	Capabilities  map[string]Support `json:"capabilities"`
	KnownIssues   []KnownIssue       `json:"known_issues"`
	Evidence      Evidence           `json:"evidence"`

	minVersion Version
	maxVersion Version
}

type VersionContract struct {
	MinInclusive      string   `json:"min_inclusive"`
	MaxExclusive      string   `json:"max_exclusive"`
	Parser            string   `json:"parser"`
	DocumentedRelease []string `json:"documented_releases"`
	TestedBuilds      []string `json:"tested_builds"`
}

type SelectionContract struct {
	LogicalAPI         string `json:"logical_api"`
	Method             string `json:"method"`
	Path               string `json:"path"`
	VersionField       string `json:"version_field"`
	MixedVersionPolicy string `json:"mixed_version_policy"`
	UnknownPolicy      string `json:"unknown_version_policy"`
}

type TransportContract struct {
	Scheme           string `json:"scheme"`
	ManagementPort   int    `json:"management_port"`
	LoginPath        string `json:"login_path"`
	TokenHeader      string `json:"token_header"`
	WhoAmIPath       string `json:"whoami_path"`
	LogoutPath       string `json:"logout_path"`
	FluxQueryPath    string `json:"flux_query_path"`
	HostHeaderPolicy string `json:"host_header_policy"`
}

type KnownIssue struct {
	ID         string `json:"id"`
	Effect     string `json:"effect"`
	Mitigation string `json:"mitigation"`
}

type Evidence struct {
	Status                string   `json:"status"`
	OfficialSources       []string `json:"official_sources"`
	APIReferenceAccess    string   `json:"api_reference_access"`
	FixtureClassification string   `json:"fixture_classification"`
	SandboxCertified      bool     `json:"sandbox_certified"`
}

// Supports reports whether the four-part product version lies in the profile's
// half-open range. Build suffixes do not affect selection.
func (p *Profile) Supports(version Version) bool {
	return version.Compare(p.minVersion) >= 0 && version.Compare(p.maxVersion) < 0
}

func loadProfile(path string) (*Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open profile: %w", err)
	}
	defer file.Close()

	var value Profile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := value.validate(); err != nil {
		return nil, fmt.Errorf("validate profile: %w", err)
	}
	return &value, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return fmt.Errorf("decode trailing JSON: %w", err)
	default:
		return fmt.Errorf("profile contains multiple JSON values")
	}
}

func (p *Profile) validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q", SchemaVersion)
	}
	if !slices.Contains(validProfileIDs, p.ProfileID) {
		return fmt.Errorf("profile_id %q is unsupported", p.ProfileID)
	}
	if p.Version.Parser != versionParser {
		return fmt.Errorf("version.parser must be %q", versionParser)
	}

	minVersion, err := ParseVersion(p.Version.MinInclusive)
	if err != nil {
		return fmt.Errorf("version.min_inclusive: %w", err)
	}
	maxVersion, err := ParseVersion(p.Version.MaxExclusive)
	if err != nil {
		return fmt.Errorf("version.max_exclusive: %w", err)
	}
	if minVersion.Suffix() != "" || maxVersion.Suffix() != "" {
		return fmt.Errorf("profile range boundaries must not contain build suffixes")
	}
	if minVersion.Compare(maxVersion) >= 0 {
		return fmt.Errorf("version range must be non-empty")
	}
	p.minVersion = minVersion
	p.maxVersion = maxVersion

	if err := p.validateVersions("documented_releases", p.Version.DocumentedRelease); err != nil {
		return err
	}
	if err := p.validateVersions("tested_builds", p.Version.TestedBuilds); err != nil {
		return err
	}
	if p.Evidence.SandboxCertified && len(p.Version.TestedBuilds) == 0 {
		return fmt.Errorf("sandbox_certified profile must contain at least one tested_build")
	}

	if p.Selection.LogicalAPI != versionDiscoveryAPI ||
		p.Selection.Method != "GET" ||
		p.Selection.Path != "/vdc/nodes" ||
		p.Selection.VersionField != "node[].version" ||
		p.Selection.MixedVersionPolicy != "intersection-only-no-interval-derived-rates" ||
		p.Selection.UnknownPolicy != "reject" {
		return fmt.Errorf("selection contract does not match the supported bootstrap policy")
	}

	if err := p.Transport.validate(); err != nil {
		return err
	}
	if len(p.Capabilities) == 0 {
		return fmt.Errorf("capabilities must not be empty")
	}
	for name, support := range p.Capabilities {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("capability name must not be empty")
		}
		if !support.valid() {
			return fmt.Errorf("capability %q has unsupported value %q", name, support)
		}
	}
	if err := validateKnownIssues(p.KnownIssues); err != nil {
		return err
	}
	if err := p.Evidence.validate(); err != nil {
		return err
	}
	return nil
}

func (p *Profile) validateVersions(field string, versions []string) error {
	seen := make(map[string]struct{}, len(versions))
	for _, value := range versions {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("version.%s contains duplicate %q", field, value)
		}
		seen[value] = struct{}{}

		version, err := ParseVersion(value)
		if err != nil {
			return fmt.Errorf("version.%s contains invalid value: %w", field, err)
		}
		if !p.Supports(version) {
			return fmt.Errorf("version.%s value %q is outside the profile range", field, value)
		}
	}
	return nil
}

func (t TransportContract) validate() error {
	switch {
	case t.Scheme != "https":
		return fmt.Errorf("transport.scheme must be https")
	case t.ManagementPort != 4443:
		return fmt.Errorf("transport.management_port must be 4443")
	case t.LoginPath != "/login":
		return fmt.Errorf("transport.login_path must be /login")
	case t.TokenHeader != "X-SDS-AUTH-TOKEN":
		return fmt.Errorf("transport.token_header is unsupported")
	case t.WhoAmIPath != "/user/whoami":
		return fmt.Errorf("transport.whoami_path must be /user/whoami")
	case t.LogoutPath != "/logout":
		return fmt.Errorf("transport.logout_path must be /logout")
	case t.FluxQueryPath != "/flux/api/external/v2/query":
		return fmt.Errorf("transport.flux_query_path is unsupported")
	}

	validPolicies := []string{
		"legacy",
		"accepted-server-names-required-for-proxy",
		"conditional-or-persisted",
	}
	if !slices.Contains(validPolicies, t.HostHeaderPolicy) {
		return fmt.Errorf("transport.host_header_policy %q is unsupported", t.HostHeaderPolicy)
	}
	return nil
}

func validateKnownIssues(issues []KnownIssue) error {
	seen := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		if issue.ID == "" || issue.Effect == "" || issue.Mitigation == "" {
			return fmt.Errorf("known issue id, effect, and mitigation are required")
		}
		if _, exists := seen[issue.ID]; exists {
			return fmt.Errorf("duplicate known issue %q", issue.ID)
		}
		seen[issue.ID] = struct{}{}
	}
	return nil
}

func (e Evidence) validate() error {
	if e.Status != "documentation-verified" && e.Status != "documentation-partial" &&
		e.Status != "sandbox-verified" {
		return fmt.Errorf("evidence.status %q is unsupported", e.Status)
	}
	if len(e.OfficialSources) == 0 {
		return fmt.Errorf("evidence.official_sources must not be empty")
	}
	seen := make(map[string]struct{}, len(e.OfficialSources))
	for _, source := range e.OfficialSources {
		parsed, err := url.ParseRequestURI(source)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("evidence source %q must be an absolute HTTPS URL", source)
		}
		if _, exists := seen[source]; exists {
			return fmt.Errorf("duplicate evidence source %q", source)
		}
		seen[source] = struct{}{}
	}
	if e.APIReferenceAccess != "downloaded-and-reviewed" &&
		e.APIReferenceAccess != "listed-by-dell-login-required" {
		return fmt.Errorf("evidence.api_reference_access %q is unsupported", e.APIReferenceAccess)
	}
	if e.FixtureClassification != fixtureClassSynthetic &&
		e.FixtureClassification != fixtureClassSandbox {
		return fmt.Errorf("evidence.fixture_classification %q is unsupported", e.FixtureClassification)
	}
	if e.SandboxCertified {
		if e.Status != "sandbox-verified" {
			return fmt.Errorf("sandbox_certified profile must have sandbox-verified evidence status")
		}
		if e.FixtureClassification != fixtureClassSandbox {
			return fmt.Errorf("sandbox_certified profile must use redacted sandbox fixtures")
		}
		if e.APIReferenceAccess != "downloaded-and-reviewed" {
			return fmt.Errorf("sandbox_certified profile requires a reviewed API reference")
		}
	} else if e.Status == "sandbox-verified" ||
		e.FixtureClassification == fixtureClassSandbox {
		return fmt.Errorf("sandbox evidence cannot be declared without sandbox certification")
	}
	return nil
}

func (s Support) valid() bool {
	switch s {
	case SupportNative, SupportDerived, SupportConditional, SupportUnavailable:
		return true
	default:
		return false
	}
}

func profileFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read profiles directory: %w", err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "ecs-") ||
			filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no ecs-*.json profiles found in %s", dir)
	}
	return paths, nil
}
