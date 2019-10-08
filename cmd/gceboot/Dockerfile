ARG GOLANG_VERSION=1.13
FROM golang:${GOLANG_VERSION} as build

WORKDIR /bigmachine

ADD . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo \
    -o /go/bin/gceboot \
    cmd/gceboot/gceboot.go

FROM scratch
COPY --from=build /go/bin/gceboot /

USER 999

ENTRYPOINT ["/gceboot","--log=debug"]
