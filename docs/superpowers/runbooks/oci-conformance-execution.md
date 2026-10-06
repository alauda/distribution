# OCI Conformance Execution Runbook

## Goal

Use this runbook when rerunning local OCI conformance for the source-built registry in this repository and you want to avoid the environment mistakes and misreads that already happened once.

This runbook is about execution discipline, not feature implementation.

## Scope

- Target repository: `distribution`
- Target runner output:
  - `tests/oci-conformance-results-local-source/results.yaml`
  - `tests/oci-conformance-results-local-source/junit.xml`
  - `tests/oci-conformance-results-local-source/report.html`
- Target config: `tests/conf-local-oci-conformance.yml`
- Related history:
  - `tests/official-registry-e2e-execution-record.md`
  - `tests/oci-conformance-case-checklist.md`

## What This Avoids

- Mistaking an old registry process for the newly built source registry
- Reading cluster results as if they represent the latest local code
- Treating skipped OCI cases as failures
- Reopening already-fixed issues such as:
  - `referrers`
  - `OCI-Subject`
  - invalid digest format
  - out-of-order final `PUT`
  - `sha512 blob mount`
  - `non-distributable layers`

## Source Of Truth

For latest code validation, trust only:

- local source registry started from this repo
- local OCI conformance pointed at `127.0.0.1:5006`

Do not use the already deployed cluster registry as proof that current local fixes work. It may still be running older code.

## Exact Execution Flow

### 1. Verify test prerequisites

- Config exists: `tests/conf-local-oci-conformance.yml`
- Result directory exists: `tests/oci-conformance-results-local-source`
- Case list reference exists: `tests/oci-conformance-case-checklist.md`

### 2. Stop any old local registry first

This is mandatory.

If an old process is still listening on `5006` or `5007`, you can get a false result: the new `go run` fails to bind, but conformance still hits the old process and shows stale failures.

Check:

```bash
lsof -nP -iTCP:5006 -sTCP:LISTEN
lsof -nP -iTCP:5007 -sTCP:LISTEN
```

If something is listening, stop it before continuing.

### 3. Start the local source registry

```bash
go run ./cmd/registry serve tests/conf-local-oci-conformance.yml
```

If you run it in background, make sure logs are captured and the process really started.

### 4. Verify the new process is the one actually running

Do not skip this.

Check both:

```bash
curl -fsS http://127.0.0.1:5006/v2/
lsof -nP -iTCP:5007 -sTCP:LISTEN
```

Expected:

- `curl` returns successfully
- debug port `5007` is owned by the newly started registry process

If start-up logs show `bind: address already in use`, stop and fix the process conflict first. Do not run conformance yet.

### 5. Run the local OCI conformance suite

```bash
OCI_REGISTRY="127.0.0.1:5006" \
OCI_TLS="disabled" \
OCI_REPO1="oci-conformance/distribution-test/repo1" \
OCI_REPO2="oci-conformance/distribution-test/repo2" \
OCI_RESULTS_DIR="tests/oci-conformance-results-local-source" \
OCI_LOG="warn" \
go run github.com/opencontainers/distribution-spec/conformance@latest
```

### 6. Verify the result from artifacts, not memory

Read:

- `tests/oci-conformance-results-local-source/results.yaml`
- `tests/oci-conformance-results-local-source/junit.xml`

Expected final summary for the known-good state in this repo:

- `OCI Conformance Test: Pass`
- `687` total checks
- `679` passed
- `8` skipped

## Expected Skips

The current known-good result contains skipped cases that are not failures.

See `tests/oci-conformance-case-checklist.md` for the exact list.

Main skip categories:

- `blob-post-only`
  - registry does not support POST-with-body shortcut
  - runner falls back to `POST -> PUT`
- `blob-mount-anonymous`
  - anonymous mount is not completed via direct `201 mounted`
  - runner falls back to ordinary upload flow
- `blob-post-cancel`
  - upload cancel is disabled by configuration

If those are the only skips, the run is still healthy.

## Red Flags

If you see any of these, do not trust the result yet:

- conformance fails with old known-bad patterns immediately after a code fix
- startup log contains `bind: address already in use`
- `5006` responds but the new process never successfully bound `5007`
- cluster endpoint and local source endpoint are being mixed in the same conclusion
- someone claims failure based only on console output without checking `results.yaml` or `junit.xml`

## Known Historical Root Causes

These were real failures that were fixed in code and should not be re-debugged unless a fresh regression proves otherwise:

- `referrers` route/handler support
- `OCI-Subject` on manifest/index GET and PUT
- invalid digest format should return `DIGEST_INVALID`
- final out-of-order `PUT` chunk should return `416`
- `sha512` cross-repo blob mount alias handling
- `non-distributable layers` validation behavior

If they reappear, first suspect execution environment drift before assuming the code regressed.

## After A Fresh Failure

Use this order:

1. Confirm you were hitting the new local source registry, not an old process
2. Confirm the failure is present in `results.yaml` or `junit.xml`
3. Confirm it is not an expected skip
4. Compare the failing case against `tests/oci-conformance-case-checklist.md`
5. Only then start code-level debugging

## Minimal Success Criteria

You can say the local OCI conformance rerun is successful only when all are true:

- new local registry process started cleanly
- `curl http://127.0.0.1:5006/v2/` succeeds
- conformance command completed
- `results.yaml` shows pass state
- skips are only the expected non-failure skips
