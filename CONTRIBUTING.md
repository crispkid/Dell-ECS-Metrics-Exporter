# Contributing

Contributions are welcome through the repository's normal pull-request process.
Do not include credentials, private endpoints, raw ECS responses, Inventory
data, or customer identifiers in issues, commits, fixtures, logs, or CI
artifacts. Security reports follow `SECURITY.md`.

## Development workflow

1. Read `AGENTS.md`, `PROJECT.md`, `SPECIFICATION.md`,
   `DELL_ECS_API_MAPPING.md`, and the active governed change.
2. Create a focused change with tests for success, failure, boundary,
   authorization, and compatibility behavior as applicable.
3. Update the specification, mapping, Profiles, fixtures, documentation, and
   traceability when a public contract or ECS response contract changes.
4. Run:

   ```bash
   ./HARNESS/harness.sh selftest
   ./HARNESS/harness.sh doctor
   ./HARNESS/harness.sh verify
   bash -lc 'source scripts/go-env.sh; go test -race -count=1 -timeout=3m ./...'
   ```

5. Report checks that actually ran and all blocked/skipped prerequisites.

Code must remain formatted with `gofmt`, pass `go vet`, maintain at least 80%
application-package coverage, and preserve backwards compatibility unless a
governed breaking change is approved.

## Dell ECS compatibility evidence

Synthetic fixtures and mock servers are component evidence only. They cannot
populate Profile `tested_builds` or certify an ECS version. Real response
evidence must come from an authorized isolated environment, be minimized and
redacted, identify the exact four-part ECS build, and receive Project Maintainer
review. Credential/TLS/CI/signing changes also require Security Reviewer review.

Unknown ECS versions remain fail-closed. Do not add nearest-version fallback,
invent units or fields, or generalize a Dell ECS CE limitation to physical ECS.

## Commit and release scope

Use clear commits and Semantic Versioning. Do not create a release, tag, publish
an image/chart, deploy, or modify external systems as part of a contribution
unless that action was explicitly authorized. Production releases follow
`docs/RELEASE_CHECKLIST.md`.
