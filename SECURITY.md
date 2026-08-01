# Security Policy

## Supported versions

Only the latest published release receives security fixes until a second stable
minor release exists. Documentation/component Profiles are not a claim that an
exact Dell ECS build is production-certified.

## Reporting a vulnerability

Do not open a public issue containing credentials, private endpoints, raw ECS
responses, Inventory data, or exploit details. Report the issue privately to
the repository security contact configured in the hosting platform. Include the
affected version, impact, minimal reproduction, and whether credentials or data
may have been exposed.

The Project Maintainer coordinates triage. Authentication, authorization,
credential, CI identity, artifact-signing, or production-deployment issues also
require the Security Reviewer. Acknowledgement and remediation timelines are
set by the operating organization because no public support SLA is currently
declared.

## Release security policy

- Critical and High findings block a release unless a documented exception has
  an owner, justification, compensating controls, and expiry date.
- Release images, archives, Helm charts, checksums, SBOMs, and provenance must
  refer to the same Git commit.
- Every published OCI platform is scanned before the multi-architecture image
  is pushed and receives a platform-specific SBOM attestation.
- Published images are signed keylessly with GitHub OIDC/Sigstore and consumed
  by immutable digest.
- Production credentials and raw responses are prohibited from repository,
  workflow logs, caches, artifacts, fixtures, and release assets.
- Dependency licenses must resolve to the approved SPDX allowlist
  (`Apache-2.0`, `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`) or have a
  documented legal/security exception before release.

The production license gate uses OSV-Scanner/deps.dev and transmits dependency
package names, versions, and ecosystem identifiers; it does not transmit source
code. Organizations that prohibit this metadata exchange must provide and
document an approved offline equivalent.
