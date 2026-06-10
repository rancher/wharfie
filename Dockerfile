# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.23 AS infra

RUN apk -U --no-cache add bash git gcc musl-dev zlib-dev zlib-static zstd gzip alpine-sdk binutils-gold
WORKDIR /go/src/github.com/rancher/wharfie

FROM infra AS build
ARG TAG
ARG DIRTY
ARG TARGETOS
ARG TARGETARCH
ENV TAG=${TAG} DIRTY=${DIRTY}

COPY --parents .git go.mod go.sum main.go pkg/ scripts/ ./
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    ./scripts/build

FROM scratch AS binary
COPY --from=build /go/src/github.com/rancher/wharfie/bin /bin/

FROM infra AS collect
ARG TARGETOS
ARG TARGETARCH
RUN mkdir -p /image/etc/ssl/certs /image/bin && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt
COPY --from=build /go/src/github.com/rancher/wharfie/bin/wharfie-$TARGETARCH /image/bin/wharfie

FROM scratch AS image
COPY --from=collect /image /
ENTRYPOINT ["/bin/wharfie"]
