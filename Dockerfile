FROM golang:1.11-alpine AS build

ENV DISTRIBUTION_DIR /go/src/github.com/docker/distribution
ENV BUILDTAGS include_oss include_gcs

RUN set -ex \
    && apk add --no-cache make git file

WORKDIR $DISTRIBUTION_DIR
COPY . $DISTRIBUTION_DIR
RUN CGO_ENABLED=0 make PREFIX=/go clean binaries && file ./bin/registry | grep "statically linked"

FROM alpine

COPY --from=build /go/src/github.com/docker/distribution/bin/registry /bin/registry
COPY cmd/registry/config-dev.yml /etc/docker/registry/config.yml
COPY cmd/registry/config-alauda.yml /etc/docker/registry/config-alauda.yml
COPY cmd/registry/init.sh /init.sh

RUN chmod +x /init.sh

VOLUME ["/var/lib/registry"]
EXPOSE 5000
ENTRYPOINT ["registry"]
CMD ["serve", "/etc/docker/registry/config.yml"]
