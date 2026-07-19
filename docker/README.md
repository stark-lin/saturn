<!-- This document explains Docker support files for Saturn development. -->
# Docker Support

The root `docker-compose.yml` is the default development topology for Saturn. This directory is reserved for Docker-specific configuration that does not belong in application packages.

`config.json` is copied into the application image as `/app/config.json` and uses Compose service names for PostgreSQL and Redis. File blobs are stored under `/app/objects`.

`postgres/init.sh` is mounted into the official PostgreSQL image's first-run initialization directory. For a new empty data volume the image creates the configured bootstrap login; the script then creates separate owner/runtime roles, executes all repository migrations as the non-login owner, and applies `postgres/runtime_grants.sql`. Existing volumes are never rebuilt by application startup. The verification SQL files are exercised by `scripts/test-postgres-bootstrap.sh` and CI.

GitHub Actions publishes the root Dockerfile to `ghcr.io/stark-lin/saturn`. The embedded `config.json` is a development default only. Deployments using a published image must mount a production configuration over `/app/config.json` and persist `/app/objects`.
