# Production Release Checklist

Every item is required for a production release unless a time-bounded security
exception names an owner, justification, compensating control, and expiry date.
Do not mark blocked or skipped checks as passed.

## Governance and source

- [ ] Release scope has an approved change ID and traceability is current.
- [ ] Project Maintainer and Security Reviewer are named and have reviewed the
      final diff and release evidence.
- [ ] All source files are committed, the worktree is clean, and the tag points
      to the reviewed commit on the configured remote.
- [ ] Release version follows Semantic Versioning and migration/release notes
      describe public API, metric, configuration, and deployment changes.

## Compatibility

- [ ] Exact production ECS `3.8.1.4` passes the protected read-only
      certification workflow with `UP` readiness.
- [ ] Exact test ECS CE `3.8.0.3` passes its applicable Management API suite;
      DEGRADED is accepted only when the sole positive collector-error series
      is `node-resources`; its Flux 503 limitation is recorded as CE-only and
      not generalized.
- [ ] All required API functions are present in every Profile's
      `shared_validated_capabilities` under the
      `shared-live-any-target-version` policy.
- [ ] Version-specific differences (`unavailable` capability, Flux interval
      policy and Host Header behavior) remain enforced; exact-build smoke in the
      release workflow validates the deployment rather than repeating shared
      feature certification.
- [ ] Unknown versions remain fail-closed and quota `null` semantics are
      preserved.

## Verification and capacity

- [ ] `./HARNESS/harness.sh selftest`, `doctor`, and `verify` pass from the
      release commit.
- [ ] Fresh race, coverage, CI policy, Helm lint/template, kubeconform, and
      container non-root/read-only startup gates pass.
- [ ] Synthetic 10-cluster/100-node/10,000-bucket precheck passes.
- [ ] Deployed target-scale evidence records metrics and Inventory p95, RSS,
      CPU, response size, and expected scale without endpoints or raw data.
- [ ] Deployed E2E proves liveness, `UP` readiness, authentication rejection,
      all Inventory collections, exact ECS version, and required metrics.

## Security and supply chain

- [ ] Credential scan, module checksum verification, `govulncheck`, source
      dependency scan, linux/amd64 and linux/arm64 image scans, and license
      review have no unapproved Critical/High findings.
- [ ] Release archives, Helm package, OCI image, metadata, checksums, SBOMs, and
      attestations refer to the same full Git commit.
- [ ] Each OCI platform manifest has its own SBOM attestation, and the signed
      multi-architecture index references exactly the approved platform
      digests.
- [ ] GitHub Actions are pinned to full commit SHAs and protected environments
      have required reviewers.
- [ ] The same full commit is staged before tagging in the protected E2E and
      target-scale environments; exporter build metadata matches the tag SHA.
- [ ] OCI image and `SHA256SUMS` are signed keylessly; signer workflow identity,
      issuer, digest, checksums, SBOM, and provenance are verified before use.
- [ ] No credential, token, private endpoint, raw response, Inventory name, or
      personal data exists in Git, logs, artifacts, SBOM, or evidence.

## Deployment and operations

- [ ] Production uses an immutable image digest or verified release archive.
- [ ] TLS verification uses the correct CA, or an explicit self-signed-certificate
      exception records the `tls.verify: false` risk, startup warning and
      restricted network path; the ECS account is least privilege.
- [ ] Inventory API is authenticated; exposure, reverse proxy, firewall, and
      NetworkPolicy are reviewed.
- [ ] Kubernetes ingress selectors and DNS/ECS egress CIDRs are
      environment-specific; documentation-only `192.0.2.0/24` is replaced.
- [ ] Resource limits, PDB, probes, Prometheus scrape, alert rules, retention,
      backup, rotation, rollback, and the production runbook are tested.
- [ ] Bare Metal service hardening and file ownership pass
      `deploy/bare-metal/verify.sh`, when that deployment mode is used.

## Approval

- [ ] Evidence locations and retention dates are recorded.
- [ ] Remaining limitations and accepted exceptions are listed.
- [ ] Project Maintainer approval: name, date, release/commit.
- [ ] Security Reviewer approval: name, date, release/commit.

The automated entry point is
`RELEASE_VERSION=v1.0.0 ./scripts/release-check.sh`. Exit code 3 means a required
prerequisite is absent and therefore blocks the release.
