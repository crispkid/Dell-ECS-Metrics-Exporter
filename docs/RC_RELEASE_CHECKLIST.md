# Release Candidate Checklist

This checklist applies only to Semantic Version tags with a prerelease suffix, such as
`v1.0.0-rc.1`. A release candidate is intended for evaluation and integration testing. It is
not a production approval and does not satisfy the stable-release checklist by inheritance.

## Required RC gates

- [ ] The governed change, changelog, README, compatibility matrix and release notes identify
      the exact RC version and its known limitations.
- [ ] The release commit is tracked, reviewed for unrelated changes and secrets, and tagged with
      a v-prefixed prerelease Semantic Version.
- [ ] Harness self-test, doctor, deterministic verification and a fresh race suite pass.
- [ ] Container non-root/read-only startup, Kubernetes schema validation, synthetic
      10-cluster/100-node/10,000-bucket validation, source dependency scanning and
      dependency-license policy pass in the tag workflow.
- [ ] linux/amd64 and linux/arm64 release images pass High/Critical vulnerability scanning.
- [ ] Archives, Helm package, OCI image, checksums, source/image SBOMs, signatures and OCI
      provenance are generated from the tagged full commit. GitHub-native attestations are also
      required when the repository is public or on GitHub Enterprise Cloud.
- [ ] GitHub marks the release as a Pre-release and the published notes state the exact missing
      appliance, deployed E2E, deployed performance and reviewer evidence.

## Stable-only gates intentionally retained

The release workflow skips the following jobs only for prerelease tags. A stable tag cannot skip
them and still reach publication:

- exact production ECS `3.8.1.4` certification;
- exact test ECS CE `3.8.0.3` compatibility validation;
- deployed end-to-end validation against the tagged commit;
- deployed 10-cluster/100-node/10,000-bucket performance validation;
- all named approvals and operational checks in `docs/RELEASE_CHECKLIST.md`.

For an RC in a private repository without GitHub Enterprise Cloud, GitHub-native artifact
attestations are unavailable by platform policy. The workflow must include the signed
`github-attestation-boundary.txt` asset and still publish source/platform SBOMs, BuildKit OCI
provenance/SBOM attestations, a keylessly signed OCI index and a keylessly signed checksum bundle.
A stable release continues to execute the GitHub-native attestation steps and fails if the
repository plan cannot support them.

The automated tag workflow classifies `vMAJOR.MINOR.PATCH-PRERELEASE` as `prerelease`, publishes
it with GitHub's Pre-release flag only after the required RC gates pass, and keeps
`vMAJOR.MINOR.PATCH` on the complete stable path.
