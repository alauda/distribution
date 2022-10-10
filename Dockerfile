# `FROM` instructions support variables that are declared by any `ARG` instructions that occur before the first `FROM`.
ARG OPS_DISTROLESS_TAG=20220718
ARG OPS_TOOLSETS_TAG=20221010150159
ARG PRIVATE_REGISTRY


FROM golang:1.11-alpine AS build

ENV DISTRIBUTION_DIR /go/src/github.com/docker/distribution
ENV BUILDTAGS include_oss include_gcs

RUN set -ex \
    && apk add --no-cache make git file

WORKDIR $DISTRIBUTION_DIR
COPY . $DISTRIBUTION_DIR
RUN CGO_ENABLED=0 make PREFIX=/go clean binaries \
    && file ./bin/registry | grep "statically linked"

COPY cmd/registry/init.sh /init.sh
RUN chmod +x /init.sh


FROM scratch AS assets

ARG ALAUDA_UID="697"
ARG ALAUDA_GID="697"

COPY --from=build /go/src/github.com/docker/distribution/bin/registry /bin/
COPY --from=build --chown=$ALAUDA_UID:$ALAUDA_GID /init.sh /
COPY --chown=$ALAUDA_UID:$ALAUDA_GID cmd/registry/config-dev.yml /etc/docker/registry/config.yml
COPY --chown=$ALAUDA_UID:$ALAUDA_GID cmd/registry/config-alauda.yml /etc/docker/registry/config-alauda.yml


FROM ${PRIVATE_REGISTRY}/ops/toolset:${OPS_TOOLSETS_TAG} AS tools
FROM ${PRIVATE_REGISTRY}/ops/distroless-static-nonroot:${OPS_DISTROLESS_TAG}
LABEL OPS_DISTROLESS_TAG="${OPS_DISTROLESS_TAG}"
LABEL OPS_TOOLSETS_TAG="${OPS_TOOLSETS_TAG}"

# 这一条命令会拷贝 /bin/bash 和 指向它的软链接 /bin/sh
COPY --from=tools /bin/ /bin/
COPY --from=tools /usr/local/bin/cat /usr/local/bin/cp \
                  /usr/local/bin/chmod /usr/local/bin/chown \
                  /usr/local/bin/echo /usr/local/bin/grep \
                  /usr/local/bin/sed /usr/local/bin/sleep \
                  /usr/local/bin/tail /usr/local/bin/pkill \
                  /usr/local/bin/find /usr/local/bin/xargs \
                  /usr/local/bin/

COPY --from=assets / /
VOLUME ["/var/lib/registry"]
EXPOSE 5000
ENTRYPOINT ["/bin/registry"]
CMD ["serve", "/etc/docker/registry/config.yml"]
