FROM golang:1.13.6-alpine3.11 AS builder
WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download
COPY main.go lookup.go /src/

ENV CGO_ENABLED 0
RUN go build ./... && go test ./... && go install ./...

FROM alpine:3.11
COPY --from=builder /go/bin/opentracing-annotator /go/bin/opentracing-annotator
EXPOSE 9411
CMD ["/go/bin/opentracing-annotator"]
