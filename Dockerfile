FROM golang:1-alpine AS builder
ARG VERSION=dev
RUN apk add --no-cache gcc musl-dev protobuf protobuf-dev && \
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
RUN sqlc generate && \
    protoc --go_out=paths=source_relative:. protos/messages.proto && \
    CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/tsukinoko-kun/pogo/metadata.Version=${VERSION}" -o server ./bin/server/

FROM alpine:latest
WORKDIR /pogo
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/server /opt/pogo/server
ENTRYPOINT ["/opt/pogo/server"]
