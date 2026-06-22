<!-- This document explains Docker support files for Saturn development. -->
# Docker Support

The root `docker-compose.yml` is the default development topology for Saturn. This directory is reserved for Docker-specific configuration that does not belong in application packages.

`config.json` is copied into the application image as `/app/config.json` and uses Compose service names for PostgreSQL and Redis. File blobs are stored under `/app/objects`.

GitHub Actions publishes the root Dockerfile to `ghcr.io/stark-lin/saturn`. The embedded `config.json` is a development default only. Deployments using a published image must mount a production configuration over `/app/config.json` and persist `/app/objects`.
