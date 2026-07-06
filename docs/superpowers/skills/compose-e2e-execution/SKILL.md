---
name: compose-e2e-execution
description: Use when starting or verifying the local compose E2E registry environment in this repository, especially if docker compose shows the registry as unhealthy, host port 5000 behaves unexpectedly, or push.sh may be hitting the wrong endpoint.
---

# Compose E2E Execution

## Overview

This skill prevents false conclusions when validating the local compose E2E registry. The key rule is: do not trust Docker health or host port `5000` until you verify the real service path.

## When to Use

- running `docker compose -f tests/docker-compose-e2e-cloud-storage.yml up`
- checking whether compose registry is actually usable
- running `tests/push.sh` against the compose environment
- debugging `unhealthy` compose registry state

## Quick Path

1. Ensure `tests/miniodata/distribution` exists
2. Start compose env with a long timeout
3. If registry is `unhealthy`, inspect health details and logs
4. Verify `http://127.0.0.1:5001/debug/health`
5. Verify container-internal `http://127.0.0.1:5000/v2/`
6. Check whether host `5000` is really your registry
7. If not, proxy host `5002 -> registry:5000`
8. Run `tests/push.sh` against the verified endpoint

## Known Pitfalls

- registry healthcheck uses `curl`, but the built image may not contain `curl`
- host `5000` may be occupied by `ControlCe` on this machine
- compose build can take a long time because the local build context is large

## Safe Commands

```bash
docker compose -f tests/docker-compose-e2e-cloud-storage.yml up -d --wait
docker compose -f tests/docker-compose-e2e-cloud-storage.yml ps -a
docker compose -f tests/docker-compose-e2e-cloud-storage.yml logs --no-color registry
curl -fsS http://127.0.0.1:5001/debug/health
docker exec tests-registry-1 /bin/sh -c 'wget -qO- http://127.0.0.1:5000/v2/'
lsof -nP -iTCP:5000 -sTCP:LISTEN
```

Proxy fallback:

```bash
docker run -d --name compose-registry-proxy --network tests_default -p 5002:5000 alpine/socat -d -d TCP-LISTEN:5000,fork,reuseaddr TCP:registry:5000
E2E_HEALTHCHECK_URL="http://127.0.0.1:5001/debug/health" sh tests/push.sh 127.0.0.1:5002
```

## References

- `docs/superpowers/runbooks/compose-e2e-execution.md`
- `tests/official-registry-e2e-execution-record.md`
