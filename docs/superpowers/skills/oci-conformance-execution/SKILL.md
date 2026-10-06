---
name: oci-conformance-execution
description: Use when rerunning local OCI conformance for this distribution repository, especially if results may be polluted by an old registry process, mixed local-vs-cluster endpoints, or confusion about skipped blob upload and anonymous mount cases.
---

# OCI Conformance Execution

## Overview

This skill prevents false conclusions when rerunning OCI conformance in this repository. The main rule is simple: do not trust any result until you have proven the runner hit the newly started local source registry.

## When to Use

- rerunning `OCI conformance` after code changes
- investigating whether a fix really took effect
- seeing old failure patterns that should already be fixed
- checking whether a skipped OCI case is a real failure

Do not use this for code implementation details. Use it for execution discipline and result interpretation.

## Quick Path

1. Stop any old process on `5006` and `5007`
2. Start local source registry with `tests/conf-local-oci-conformance.yml`
3. Verify `curl http://127.0.0.1:5006/v2/` works
4. Verify the new process owns `5007`
5. Run OCI conformance against `127.0.0.1:5006`
6. Verify final state from `results.yaml` and `junit.xml`

## Commands

```bash
lsof -nP -iTCP:5006 -sTCP:LISTEN
lsof -nP -iTCP:5007 -sTCP:LISTEN
go run ./cmd/registry serve tests/conf-local-oci-conformance.yml
curl -fsS http://127.0.0.1:5006/v2/
OCI_REGISTRY="127.0.0.1:5006" OCI_TLS="disabled" OCI_REPO1="oci-conformance/distribution-test/repo1" OCI_REPO2="oci-conformance/distribution-test/repo2" OCI_RESULTS_DIR="tests/oci-conformance-results-local-source" OCI_LOG="warn" go run github.com/opencontainers/distribution-spec/conformance@latest
```

## What To Trust

Trust only:

- local source registry started from this repository
- `tests/oci-conformance-results-local-source/results.yaml`
- `tests/oci-conformance-results-local-source/junit.xml`

Do not treat the deployed cluster registry as proof of the latest local code behavior.

## Expected Healthy Result

- `OCI Conformance Test: Pass`
- `687` total
- `679` passed
- `8` skipped

## Expected Skips

- `blob-post-only`: POST-with-body shortcut unsupported, runner falls back to `POST -> PUT`
- `blob-mount-anonymous`: anonymous mount falls back to normal upload flow
- `blob-post-cancel`: upload cancel disabled by config

These are not failures.

## Red Flags

- `bind: address already in use`
- old failure patterns reappear immediately after a fix
- local and cluster endpoints are mixed in one conclusion
- result is judged from console text only, without checking artifacts

## References

- `docs/superpowers/runbooks/oci-conformance-execution.md`
- `tests/oci-conformance-case-checklist.md`
- `tests/official-registry-e2e-execution-record.md`
