proto:
    protoc --go_out=paths=source_relative:. protos/messages.proto

db:
    sqlc generate

prebuild:
    @just proto
    @just db

build:
    @just prebuild
    go build ./bin/pogo/

install:
    @just prebuild
    go install ./bin/pogo/

server:
    @just prebuild
    PORT=4321 DATABASE_URL=postgres://pogo:pogo@localhost:5432/pogo go run ./bin/server/
