FROM alpine:3.15 AS base
RUN apk add -U ca-certificates
RUN mkdir -p /image/etc/ssl/certs /image/bin && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt
COPY bin/wharfie /image/bin
COPY bin/plugins /image/bin/plugins
COPY package/config.yaml /image/etc/

FROM scratch
COPY --from=base /image /
ENTRYPOINT ["/bin/wharfie"]
