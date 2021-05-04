ARG GOLANG=golang:1.16.2-alpine3.12
FROM ${GOLANG} AS plugins
ENV GO111MODULE=on CGO_ENABLED=0
RUN apk -U --no-cache add bash git gcc musl-dev zlib-dev zlib-static zstd gzip alpine-sdk binutils-gold && \
    mkdir -p /image/bin/plugins /go/src/github.com/kubernetes
RUN cd /go/src/github.com/kubernetes && \
    git clone --depth 1 https://github.com/kubernetes/cloud-provider-aws.git && \
    cd cloud-provider-aws && \
    go build -ldflags="-w -s" -o=/image/bin/plugins/ecr cmd/ecr-credential-provider/*.go
RUN cd /go/src/github.com/kubernetes && \
    git clone --depth 1 https://github.com/kubernetes/cloud-provider-gcp.git && \
    cd cloud-provider-gcp && \
    go build -ldflags="-w -s" -o=/image/bin/plugins/gcr cmd/auth-provider-gcp/*.go

FROM alpine:3.12 AS base
RUN apk add -U ca-certificates
RUN mkdir -p /image/etc/ssl/certs /image/bin && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt
COPY --from=plugins /image /image/
COPY bin/wharfie /image/bin/
COPY package/config.yaml /image/etc/

FROM scratch
COPY --from=base /image /
ENTRYPOINT ["/bin/wharfie"]
CMD []
