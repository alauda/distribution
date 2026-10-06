#!/bin/sh

set -u

DEFAULT_REGISTRY_PORT=${E2E_REGISTRY_PORT:-5000}
REGISTRY_HOST=$1
case "$REGISTRY_HOST" in
    *:*)
        ;;
    *)
        REGISTRY_HOST="$REGISTRY_HOST:$DEFAULT_REGISTRY_PORT"
        ;;
esac
IMAGE_REF="$REGISTRY_HOST/distribution/hello-world:latest"
HEALTHCHECK_URL=${E2E_HEALTHCHECK_URL-http://localhost:5001/debug/health}

set +e

if [ -n "$HEALTHCHECK_URL" ]; then
    TIMEOUT=5
    while [ "$TIMEOUT" -gt 0 ]; do
        STATUS=$(curl --insecure -s -o /dev/null -w '%{http_code}' "$HEALTHCHECK_URL")
        echo "$STATUS"
        if [ "$STATUS" -eq 200 ]; then
            break
        fi
        TIMEOUT=$((TIMEOUT - 1))
        sleep 5
    done

    if [ "$TIMEOUT" -eq 0 ]; then
        echo "Distribution cannot be available within one minute."
        exit 1
    fi
fi

set -e

if [ -n "${E2E_REGISTRY_USERNAME:-}" ] && [ -n "${E2E_REGISTRY_PASSWORD:-}" ]; then
    docker login -u "$E2E_REGISTRY_USERNAME" -p "$E2E_REGISTRY_PASSWORD" "$REGISTRY_HOST"
fi

docker pull hello-world:latest
docker tag hello-world:latest "$IMAGE_REF"
docker push "$IMAGE_REF"
docker pull "$IMAGE_REF"
