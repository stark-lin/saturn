# This script runs formatting, architecture checks, and Go tests for Saturn.
set -eu

go tool sqlc generate
sh scripts/check-docs.sh
gofmt -w tools.go cmd internal
go run github.com/arch-go/arch-go/v2 --color no
go test ./...
