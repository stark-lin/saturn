# This Dockerfile builds and runs the Saturn HTTP server.
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/saturn ./cmd/server

FROM alpine:3.21

WORKDIR /app
COPY --from=build /out/saturn /app/saturn
COPY docker/config.json /app/config.json
COPY migrations /app/migrations
COPY web/src /app/web/src
EXPOSE 8080
CMD ["/app/saturn", "-config", "/app/config.json"]
