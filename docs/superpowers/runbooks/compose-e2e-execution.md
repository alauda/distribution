# Compose E2E Execution Runbook

## Goal

Use this runbook when you need to start the official local compose E2E environment in this repository and verify that the compose-backed registry can really serve push/pull traffic.

This runbook exists because "compose started" is not enough on this machine. Two environment-specific problems were already observed:

- the registry container can be marked `unhealthy` even when the service is fine
- host port `5000` can point to a system service instead of the compose registry

## Scope

- Compose file: `tests/docker-compose-e2e-cloud-storage.yml`
- Registry config: `tests/conf-e2e-cloud-storage.yml`
- Functional check script: `tests/push.sh`
- Historical record: `tests/official-registry-e2e-execution-record.md`

## What To Verify

You need all three layers:

1. compose services start
2. registry service is actually reachable
3. `pull -> tag -> push -> pull` works against the compose registry

Do not stop at `docker compose ps`.

## Known Machine-Specific Pitfalls

### 1. Registry healthcheck can lie

In `tests/docker-compose-e2e-cloud-storage.yml`, the registry healthcheck uses `curl`.

On the current built `tests-registry` image, `curl` is not present. Result:

- Docker marks `tests-registry-1` as `unhealthy`
- but the registry process itself may already be serving correctly on `:5000` and `:5001`

So `unhealthy` is not enough to conclude the service is broken.

### 2. Host port `5000` may not be your registry

On this machine, `lsof -nP -iTCP:5000 -sTCP:LISTEN` showed the system process `ControlCe` occupying `5000`.

That means:

- `curl http://127.0.0.1:5000/v2/` can hit the wrong service
- `tests/push.sh 127.0.0.1` can push to the wrong target

Always prove the host port is really pointing at the compose registry before trusting it.

## Execution Flow

### 1. Ensure bind mount directory exists

```bash
ls tests/miniodata
```

Expected:

- `tests/miniodata/distribution` exists

This avoids the previously observed bind mount initialization failure.

### 2. Start compose environment

```bash
docker compose -f tests/docker-compose-e2e-cloud-storage.yml up -d --wait
```

Important:

- the command may take a long time because it builds the local `tests-registry` image
- the build context in this repository is large, so allow a long timeout

### 3. Inspect actual service state

Run:

```bash
docker compose -f tests/docker-compose-e2e-cloud-storage.yml ps -a
docker compose -f tests/docker-compose-e2e-cloud-storage.yml logs --no-color registry
docker inspect tests-registry-1 --format '{{json .State.Health}}'
```

Interpretation:

- if `registry` is `unhealthy`, check whether the health log says `curl: executable file not found`
- if yes, treat it as a healthcheck tooling issue, not immediate proof of service failure

### 4. Prove the registry service is actually alive

Run both:

```bash
curl -fsS http://127.0.0.1:5001/debug/health
docker exec tests-registry-1 /bin/sh -c 'wget -qO- http://127.0.0.1:5000/v2/'
```

Expected:

- health endpoint returns `{}`
- container-internal `/v2/` returns `{}`

If both pass, the registry service is alive even if Docker health says `unhealthy`.

### 5. Check whether host port `5000` is safe to use

```bash
lsof -nP -iTCP:5000 -sTCP:LISTEN
curl -i http://127.0.0.1:5000/v2/
```

If the listener is not the compose stack, or the response is unexpected, do not use `127.0.0.1:5000` for `tests/push.sh`.

### 6. If host `5000` is polluted, create a temporary proxy

Use a clean host port such as `5002`:

```bash
docker rm -f compose-registry-proxy >/dev/null 2>&1 || true
docker run -d --name compose-registry-proxy --network tests_default -p 5002:5000 alpine/socat -d -d TCP-LISTEN:5000,fork,reuseaddr TCP:registry:5000
curl -fsS http://127.0.0.1:5002/v2/
```

If the last command returns `{}`, use `5002` as the registry endpoint.

### 7. Run functional push/pull verification

Normal path if host `5000` is valid:

```bash
E2E_HEALTHCHECK_URL="http://127.0.0.1:5001/debug/health" sh tests/push.sh 127.0.0.1
```

Safe path if you needed the proxy:

```bash
E2E_HEALTHCHECK_URL="http://127.0.0.1:5001/debug/health" sh tests/push.sh 127.0.0.1:5002
```

Success means:

- `docker pull hello-world:latest`
- `docker tag`
- `docker push`
- `docker pull` back from target registry

all complete successfully.

## Cleanup

Always clean up after the run:

```bash
docker compose -f tests/docker-compose-e2e-cloud-storage.yml down
docker rm -f compose-registry-proxy >/dev/null 2>&1 || true
docker builder prune -f
```

Optionally remove test images too if disk pressure matters.

## Minimal Success Criteria

You can say the compose E2E environment is verified only when all are true:

- compose stack started
- `minio`, `minio-init`, `redis`, and `registry` were created successfully
- registry health endpoint returned `{}`
- registry `/v2/` responded from inside the container
- `tests/push.sh` completed against the compose-backed registry

## References

- `tests/docker-compose-e2e-cloud-storage.yml`
- `tests/conf-e2e-cloud-storage.yml`
- `tests/push.sh`
- `tests/official-registry-e2e-execution-record.md`
