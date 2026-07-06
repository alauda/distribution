#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

FAKEBIN="$TMP_DIR/bin"
LOGFILE="$TMP_DIR/commands.log"
mkdir -p "$FAKEBIN"

cat > "$FAKEBIN/curl" <<'EOF'
#!/bin/sh
exit 1
EOF
chmod +x "$FAKEBIN/curl"

cat > "$FAKEBIN/docker" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$LOGFILE"
exit 0
EOF
chmod +x "$FAKEBIN/docker"

PATH="$FAKEBIN:$PATH" \
E2E_HEALTHCHECK_URL="" \
E2E_REGISTRY_USERNAME="user1" \
E2E_REGISTRY_PASSWORD="pass1" \
sh "$ROOT_DIR/tests/push.sh" registry-gateway-service.cpaas-system.svc.cluster.local:5000

if ! grep -qx 'login -u user1 -p pass1 registry-gateway-service.cpaas-system.svc.cluster.local:5000' "$LOGFILE"; then
    echo "expected docker login against registry-gateway endpoint"
    cat "$LOGFILE"
    exit 1
fi

if ! grep -qx 'tag hello-world:latest registry-gateway-service.cpaas-system.svc.cluster.local:5000/distribution/hello-world:latest' "$LOGFILE"; then
    echo "expected docker tag to preserve full registry endpoint"
    cat "$LOGFILE"
    exit 1
fi

if ! grep -qx 'push registry-gateway-service.cpaas-system.svc.cluster.local:5000/distribution/hello-world:latest' "$LOGFILE"; then
    echo "expected docker push to registry-gateway endpoint"
    cat "$LOGFILE"
    exit 1
fi

if ! grep -qx 'pull registry-gateway-service.cpaas-system.svc.cluster.local:5000/distribution/hello-world:latest' "$LOGFILE"; then
    echo "expected docker pull from registry-gateway endpoint"
    cat "$LOGFILE"
    exit 1
fi

: > "$LOGFILE"

PATH="$FAKEBIN:$PATH" \
E2E_HEALTHCHECK_URL="" \
sh "$ROOT_DIR/tests/push.sh" 127.0.0.1

if ! grep -qx 'tag hello-world:latest 127.0.0.1:5000/distribution/hello-world:latest' "$LOGFILE"; then
    echo "expected host-only input to keep default port 5000"
    cat "$LOGFILE"
    exit 1
fi
