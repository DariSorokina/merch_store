FROM golang:1.22

WORKDIR ${GOPATH}/merch_store/
COPY . ${GOPATH}/merch_store/

RUN go build -o /build ./cmd/store \
    && go clean -cache -modcache

EXPOSE 8080

CMD ["/build"]