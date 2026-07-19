# This script verifies the PostgreSQL first-run migrations and Saturn runtime permissions in a disposable container.
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
container_name="saturn-postgres-bootstrap-test-$$"

cleanup() {
    docker container rm --force "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker run --detach --rm \
    --name "$container_name" \
    --tmpfs /var/lib/postgresql/data:rw \
    --env POSTGRES_DB=saturn \
    --env POSTGRES_USER=saturn_bootstrap \
    --env POSTGRES_PASSWORD=saturn-bootstrap \
    --env SATURN_DATABASE_PASSWORD=saturn \
    --volume "$repository_root/migrations:/opt/saturn/migrations:ro" \
    --volume "$repository_root/docker/postgres:/opt/saturn/postgres:ro" \
    --volume "$repository_root/docker/postgres/init.sh:/docker-entrypoint-initdb.d/010_saturn.sh:ro" \
    postgres:17-alpine >/dev/null

ready=false
attempt=0
while [ "$attempt" -lt 60 ]; do
    if docker exec --env PGPASSWORD=saturn "$container_name" \
        psql --host 127.0.0.1 --username saturn --dbname saturn --tuples-only --no-align \
        --command "SELECT to_regclass('public.audit_logs') IS NOT NULL" 2>/dev/null | grep -qx t; then
        ready=true
        break
    fi
    attempt=$((attempt + 1))
    sleep 1
done

if [ "$ready" != true ]; then
    docker logs "$container_name"
    echo "PostgreSQL bootstrap did not become ready" >&2
    exit 1
fi

docker exec "$container_name" \
    psql --username saturn_bootstrap --dbname saturn --set ON_ERROR_STOP=1 \
    --file /opt/saturn/postgres/verify_bootstrap.sql

docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --file /opt/saturn/postgres/verify_runtime_access.sql

docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "INSERT INTO audit_logs (actor_ref_code, action, target_ref_code, result, reason, source_ip) VALUES ('SYS-00000000', 'LOGIN', 'SYS-00000000', 'FAILED', 'bootstrap_permission_test', '127.0.0.1')" \
    --command "INSERT INTO api_keys (ref_code, name, key_prefix, key_hash, scopes) VALUES ('KEY-00000001', 'permission-test', 'sat_sk_12345678', repeat('a', 64), ARRAY['data:read'])" \
    --command "UPDATE api_keys SET last_used_at = clock_timestamp() WHERE ref_code = 'KEY-00000001'"

if docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "UPDATE audit_logs SET reason = 'tampered'"; then
    echo "Saturn unexpectedly updated audit_logs" >&2
    exit 1
fi

if docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "UPDATE api_keys SET last_used_at = '2000-01-01T00:00:00Z' WHERE ref_code = 'KEY-00000001'"; then
    echo "Saturn unexpectedly bypassed the API-key lifecycle trigger" >&2
    exit 1
fi

if docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "ALTER TABLE api_keys DISABLE TRIGGER api_keys_lifecycle"; then
    echo "Saturn unexpectedly disabled a trigger" >&2
    exit 1
fi

if docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "SET session_replication_role = replica"; then
    echo "Saturn unexpectedly changed session_replication_role" >&2
    exit 1
fi

if docker exec --env PGPASSWORD=saturn "$container_name" \
    psql --host 127.0.0.1 --username saturn --dbname saturn --set ON_ERROR_STOP=1 \
    --command "SET ROLE saturn_owner"; then
    echo "Saturn unexpectedly assumed the owner role" >&2
    exit 1
fi

echo "PostgreSQL bootstrap and Saturn runtime permissions verified"
