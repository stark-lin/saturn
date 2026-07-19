# DEPLOY.md

## 1. Goal

This document records Saturn's deployment boundaries.

---

## 2. Default Deployment

Development defaults to using the root directory's `docker-compose.yml`.

The default topology includes:

```text
Go app
PostgreSQL
Redis
local filesystem storage volume
```

Redis is required for administrator browser sessions; Saturn API keys authenticate from PostgreSQL and do not allocate Redis sessions. A no-Redis Web mode is not provided.

For host-machine development, Compose publishes PostgreSQL at
`127.0.0.1:5432` and Redis at `127.0.0.1:6379`. These dependency ports are
bound to loopback only; the application remains published on port `8080`.

`docker-compose.yml` uses `depends_on` for container startup order only. Dependency readiness is enforced by the application startup wait described below, so the app container may start before PostgreSQL or Redis accepts connections, then block until both are ready or the configured readiness timeout expires.

On an empty `postgres-data` volume, the official PostgreSQL entrypoint creates the `POSTGRES_USER` bootstrap login and then sources `docker/postgres/init.sh`. The script creates the owner/runtime roles, switches to the non-login schema owner, applies every `migrations/*.sql` file in lexical order, installs the runtime grants from `docker/postgres/runtime_grants.sql`, and commits the entire operation as one transaction. PostgreSQL only runs `/docker-entrypoint-initdb.d` for an empty data directory; recreating a container with an existing volume preserves the schema and data and does not rerun migrations.

The three database identities are deliberately separate:

| Role | Login | Purpose |
| --- | --- | --- |
| `saturn_bootstrap` | yes, Compose initialization only | PostgreSQL image bootstrap superuser; never provided to the app |
| `saturn_owner` | no | Owns the database, `public` schema, tables, sequences, types, functions, and triggers |
| `saturn` | yes | Application runtime role with explicit query-level grants only |

The committed passwords are development-only Compose defaults. A real deployment must supply independent secrets for bootstrap and runtime connections and must expose only the runtime connection to the Saturn process.

Files default to using the local filesystem runtime backend. The Docker development environment mounts `/app/objects` to the `objects-data` volume; local development defaults to writing to `./objects`.

Application runtime configuration defaults to coming from `config.json`, or it can be specified via the `-config path/to/config.json` path. If `config.json` is missing during local development, the service will generate a local configuration file using built-in defaults and current `SATURN_*` environment variables upon startup; once generated, environment variables will no longer overwrite existing configurations.

`startup.readiness_timeout_seconds` defaults to `30`. During startup the application waits for PostgreSQL and Redis concurrently, but the main startup flow blocks until both dependencies are ready. The wait is bounded by this timeout; if either dependency is still unavailable when the timeout expires, startup fails fast and the HTTP server and workers are not started.

Saturn application startup never executes migrations, drops tables, or uses a schema-owner connection. It only waits for PostgreSQL and Redis, opens the runtime connection, bootstraps the singleton development administrator through normal DML when absent, wires services, and starts the HTTP server and workers. An empty or incomplete database therefore fails through normal repository initialization instead of being modified by the application.

The removed `database.drop_tables` key is no longer valid configuration. Because existing JSON files reject unknown fields, delete that key when upgrading a local configuration.

The repository commits `config.example.json` as a local configuration template; the real `config.json` is not committed. The Docker image uses `docker/config.json` copied to `/app/config.json`, and starts via `-config /app/config.json`.

If an existing configuration file fails JSON parsing, contains unknown fields, or lacks required infrastructure or authentication configuration fields, the service must fail fast, without falling back to environment variables or built-in defaults.

LLM is disabled by default. To enable an OpenAI-style provider, set `enabled=true`, `api_key`, `endpoint`, `model`, `rate_limit_per_minute`, `max_tokens`, `worker_count`, and `timeout_seconds` in the `llm` configuration section. `worker_count` controls the fixed number of workers executing provider calls concurrently; exceeding `timeout_seconds` will write a failed result. If `config.json` is missing, it can be populated via `SATURN_LLM_*` environment variables when the configuration is first generated; existing configuration files will not be overwritten by environment variables. `llm.api_key` is a secret and the real value must not be committed.

Audit logs require every HTTP source event to have a `source_ip`. It defaults to obtaining from the socket remote address; IPv6 loopback is normalized to `127.0.0.1`. When deployed behind a reverse proxy, `X-Forwarded-For` can only be used after declaring the actual trusted proxy subnets in the JSON configuration's `http.trusted_proxy_cidrs`, for example:

```json
{
  "http": {
    "addr": ":8080",
    "trusted_proxy_cidrs": ["172.18.0.0/16"]
  }
}
```

Do not configure the public internet or arbitrary source subnets as trusted proxies, otherwise clients can spoof the audit source IP.

The development configuration provides a default JWT secret and bootstraps the singleton administrator with password `admin` after schema creation. Bootstrap claims the administrator's `USR-*` RefCode from the global ObjectRef sequence and registers it with the `SYS` owner; on a clean schema this is normally `USR-00000001`. If `/app/config.json` is absent on startup, Saturn generates one with a random JWT secret and writes it to disk; production deployments should mount a persistent config at that path rather than relying on regeneration. The JWT secret and default administrator password must be replaced prior to actual deployment. Create programmatic credentials from the settings API-key page after login; never place issued `sat_sk_*` values in committed configuration.

Formal migration version tracking and in-place production upgrades are not implemented. Adding a migration changes the schema created for new empty PostgreSQL volumes, but does not mutate an existing volume. Rebuild a development database by first backing up any needed data and then deliberately replacing only the PostgreSQL data volume; never remove the shared object-storage volume as an accidental side effect. Valuable or production data requires a separately reviewed migration run using a privileged migration identity, never the `saturn` runtime role.

---

## 3. GitHub Actions CI/CD

The repository uses `.github/workflows/ci-cd.yml` for continuous integration and container delivery.

The pipeline has the following behavior:

| Event | Checks | Container result |
| --- | --- | --- |
| Pull request | sqlc generated code, formatting, `go vet`, architecture rules, all Go tests, and disposable PostgreSQL bootstrap/permission verification | Build the current-platform image without publishing it |
| Push to `main` | Same quality and PostgreSQL checks | Publish multi-platform `latest` and `sha-<commit>` images |
| Manual dispatch | Same quality and PostgreSQL checks | Build the current-platform image without publishing it |

Container publishing only starts after the quality and PostgreSQL bootstrap jobs succeed. The database job runs `scripts/test-postgres-bootstrap.sh` against a disposable PostgreSQL 17 container and verifies the complete schema, owner/runtime role attributes, exact audit permissions, trigger enforcement, and rejected bypass attempts. Published application images support `linux/amd64` and `linux/arm64`, include an SBOM, and receive a GitHub artifact provenance attestation.

The target registry and image name are:

```text
ghcr.io/stark-lin/saturn
```

The workflow authenticates to GitHub Container Registry with the job's built-in `GITHUB_TOKEN`. No repository secret is required. The repository or organization Actions settings must allow workflows to write packages. Package visibility is managed separately in GitHub Packages after the first publish.

Every push to `main` publishes a complete container package after the checks pass:

```sh
docker pull ghcr.io/stark-lin/saturn:latest
```

Each publish also creates an immutable tag from the full Git commit SHA, so a deployment can be pinned and rolled back without assigning a manual version:

```sh
docker pull ghcr.io/stark-lin/saturn:sha-<full-commit-sha>
```

The workflow triggers on all pushes to `main`; use GitHub branch protection to require pull requests if only merged changes should be publishable.

The published application image contains `docker/config.json` only as a development default and does not contain or execute schema migrations. A real deployment must provision PostgreSQL separately with an owner/migration identity, grant a distinct runtime identity, mount a production configuration at `/app/config.json`, replace the development JWT secret and administrator credentials, and use database and Redis addresses reachable from inside the container. For example:

```sh
docker run --rm \
  --name saturn \
  --publish 8080:8080 \
  --volume "$PWD/config.json:/app/config.json:ro" \
  --volume saturn-objects:/app/objects \
  ghcr.io/stark-lin/saturn:sha-<full-commit-sha>
```

This pipeline's CD boundary is a tested, versioned image in GHCR. It does not automatically restart a Docker Host because the repository does not define a target host, credentials, maintenance policy, or rollback policy.

---

## 4. Subsequent Deployment Forms

In later stages, adjusting container separation under a single Docker Host can be considered:

```text
app
PostgreSQL
Redis
local filesystem storage volume
```

Separated deployment does not change the product constraints of PostgreSQL, Redis, and Files local storage directory as default required runtime components.

It does not target Kubernetes, multi-node high availability, or public SaaS multi-tenant operation as default goals.
