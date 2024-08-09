# `FROM` instructions support variables that are declared by any `ARG` instructions that occur before the first `FROM`.
ARG OPS_DISTROLESS_TAG=latest
ARG OPS_TOOLSETS_TAG=20240314132050
ARG PRIVATE_REGISTRY


FROM golang:1.16-alpine AS build

ENV GO111MODULE=auto
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


FROM ${PRIVATE_REGISTRY}/ops/toolset:${OPS_TOOLSETS_TAG} AS tools
FROM scratch AS assets

COPY --from=tools /bin/ /bin/

COPY --from=tools /usr/local/bin/cat /usr/local/bin/echo \
                  /usr/local/bin/grep /usr/local/bin/sed \
                  /usr/local/bin/sleep /usr/local/bin/tail \
                  /usr/local/bin/cp \
                  /usr/local/bin/chmod /usr/local/bin/chown \
                  /usr/local/bin/find /usr/local/bin/xargs \
                  /usr/local/bin/mkdir \
                  /usr/local/bin/

COPY --from=build --chmod=550 /go/src/github.com/docker/distribution/bin/registry /bin/
COPY --from=build --chmod=550 /init.sh /
COPY --chmod=640 cmd/registry/config-dev.yml    /etc/docker/registry/config.yml
COPY --chmod=640 cmd/registry/config-alauda.yml /etc/docker/registry/config-alauda.yml


FROM ${PRIVATE_REGISTRY}/ops/distroless-static-nonroot:${OPS_DISTROLESS_TAG}
LABEL OPS_DISTROLESS_TAG="${OPS_DISTROLESS_TAG}"
LABEL OPS_TOOLSETS_TAG="${OPS_TOOLSETS_TAG}"

COPY --from=assets --chown=nonroot:nonroot / /

VOLUME ["/var/lib/registry"]
EXPOSE 5000
ENTRYPOINT ["/bin/registry"]
CMD ["serve", "/etc/docker/registry/config.yml"]
