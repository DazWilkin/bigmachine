ARG GOLANG_VERSION=1.13
FROM golang:${GOLANG_VERSION} as build

WORKDIR /bigmachine

ADD . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo \
    -o /go/bin/k8sboot \
    cmd/k8sboot/k8sboot.go

FROM debian@sha256:415e40a8c50d4e40bd438de4a0142d8317ce3a168ff2b0f52433db1dd5020734

COPY --from=build /go/bin/k8sboot /

ENTRYPOINT ["/k8sboot","--log=debug"]
