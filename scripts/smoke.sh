# This script smoke-checks the local Saturn HTTP surface.
set -eu

base_url="${SATURN_BASE_URL:-http://127.0.0.1:8080}"
curl -fsS "$base_url/healthz"
curl -fsS "$base_url/"
