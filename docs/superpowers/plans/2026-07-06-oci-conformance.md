# OCI Conformance Execution Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reproduce the upstream `conformance.yml` flow locally and run OCI Distribution Spec conformance against the official compose-backed registry environment.

**Architecture:** Reuse the already validated `tests/docker-compose-e2e-cloud-storage.yml` environment as the local registry under test, then run the upstream `opencontainers/distribution-spec/conformance` runner with the equivalent environment variables used by the GitHub Actions workflow. Capture generated reports, then stop the environment and clean all locally pulled/generated test images and build cache.

**Tech Stack:** Docker Compose, Go 1.25, `opencontainers/distribution-spec/conformance`, local registry + MinIO + Redis

---

### Task 1: Inspect upstream conformance path

**Files:**
- Read: `.github/workflows/conformance.yml`
- Read: `tests/conf-e2e-cloud-storage.yml`
- Read: `tests/official-registry-e2e-execution-record.md`

- [ ] **Step 1: Confirm upstream env and feature flags**
- [ ] **Step 2: Confirm local compose registry endpoint and healthcheck**
- [ ] **Step 3: Confirm cleanup requirement is preserved after execution**

### Task 2: Prepare conformance runner

**Files:**
- Create: `docs/superpowers/plans/2026-07-06-oci-conformance.md`

- [ ] **Step 1: Start the official compose-backed registry environment**
- [ ] **Step 2: Fetch/build the conformance runner locally**
- [ ] **Step 3: Verify the registry health endpoint before running tests**

### Task 3: Execute and collect evidence

**Files:**
- Read: `tests/official-registry-e2e-execution-record.md`

- [ ] **Step 1: Run the OCI conformance runner with upstream-equivalent env vars**
- [ ] **Step 2: Capture stdout, return code, and generated `report.html` / `junit.xml` / `results.yaml` if present**
- [ ] **Step 3: Note any unsupported API or feature-specific failures separately**

### Task 4: Cleanup and record

**Files:**
- Modify: `tests/official-registry-e2e-execution-record.md`

- [ ] **Step 1: Stop compose services**
- [ ] **Step 2: Remove locally pulled/generated test images and builder cache**
- [ ] **Step 3: Update the execution record with commands, results, blockers, and cleanup evidence**
