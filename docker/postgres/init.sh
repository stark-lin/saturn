# This script atomically creates Saturn database roles, applies every migration, and installs runtime grants.
set -eu

: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${SATURN_DATABASE_PASSWORD:?SATURN_DATABASE_PASSWORD is required}"

export LC_ALL=C

set -- /opt/saturn/migrations/*.sql
if [ ! -f "$1" ]; then
    echo "no Saturn migrations found under /opt/saturn/migrations" >&2
    exit 1
fi

{
    printf '%s\n' '\set ON_ERROR_STOP on'
    printf '%s\n' 'BEGIN;'
    printf '%s\n' 'CREATE ROLE saturn_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;'
    printf '%s\n' "CREATE ROLE saturn LOGIN PASSWORD :'saturn_database_password' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;"
    printf '%s\n' 'ALTER DATABASE :"saturn_database_name" OWNER TO saturn_owner;'
    printf '%s\n' 'REVOKE ALL ON DATABASE :"saturn_database_name" FROM PUBLIC;'
    printf '%s\n' 'GRANT CONNECT ON DATABASE :"saturn_database_name" TO saturn;'
    printf '%s\n' 'ALTER SCHEMA public OWNER TO saturn_owner;'
    printf '%s\n' 'REVOKE ALL ON SCHEMA public FROM PUBLIC;'
    printf '%s\n' 'SET ROLE saturn_owner;'
    for migration in "$@"; do
        printf '\\i %s\n' "$migration"
    done
    printf '%s\n' 'RESET ROLE;'
    printf '%s\n' '\i /opt/saturn/postgres/runtime_grants.sql'
    printf '%s\n' 'COMMIT;'
} | psql \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --set=saturn_database_name="$POSTGRES_DB" \
    --set=saturn_database_password="$SATURN_DATABASE_PASSWORD"
