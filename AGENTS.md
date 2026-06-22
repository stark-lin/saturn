# AGENTS.md

## Start Here

- Product scope and boundaries: `docs/PRD.md`
- Project structure and dependency rules: `docs/FILES.md`
- Module dependency summary: `docs/MODULES.md`
- API conventions and module endpoint contracts: `docs/API.md`, `docs/api/`
- Data model, permissions, deploy, and dependencies: `docs/ER.md`, `docs/PERMISSION.md`, `docs/DEPLOY.md`, `docs/DEPENDENCIES.md`
- Testing and logging standards: `docs/TESTING_LOGGING.md`
- Entry point: `cmd/server/main.go`
- App wiring and routes: `internal/app`
- Business modules: `internal/accounting`, `internal/notes`, `internal/files`, `internal/calendar`, `internal/llm`
- Lower-level service support: `internal/platform`
- SQL migrations: `migrations`
- Frontend source: `web/src`
- Development deployment: `docker-compose.yml`, `Dockerfile`, `docker/`

## Rules

- Keep changes small and focused.
- Do not mix unrelated refactors with feature work.
- For non-trivial code or architecture changes, propose a short plan first.
- Small docs, typo, formatting, or obvious local fixes can be done directly.
- If behavior, architecture, API, config, commands, or deployment changes, update docs in the same change.
- Code, identifiers, errors, logs, and comments must be in English.
- Docs under `docs/` may be Chinese for now.

## Frontend First Version

- The first frontend version must use plain native HTML.
- Do not introduce frontend frameworks, bundlers, CSS frameworks, component libraries, or layout systems unless explicitly approved.
- Do not spend effort on visual design, complex layout, responsive polish, or styling in the first version.
- Prefer browser-default controls: forms, inputs, buttons, links, lists, and tables.
- Use minimal vanilla JavaScript only when required for REST API calls, SSE, or small local interactions.
- The first version should prioritize working business flows over appearance.

## Architecture

- `internal/app` wires dependencies and routes. It does not own business rules.
- Business logic lives in module services.
- Handlers stay thin: bind input, read the actor, call services, write responses.
- Services enforce business rules, resource-level access control, and audit.
- Repos do data access only.
- Ops UI is a superuser aggregation layer; it reuses normal module services/facades and does not own privileged business paths.
- `internal/platform/search` owns global search indexing and query orchestration; source modules expose contributors/facades.
- `internal/platform` is the lower layer that services may depend on.
- `internal/platform` must not depend on business modules.
- `platform/auth` returns authorization decisions or scopes; it does not generate business SQL.
- Module repos translate access scopes into fixed, parameterized SQL helpers.

## Go

- Follow standard Go style and run `gofmt` on changed Go files.
- Prefer simple, explicit code over clever abstractions.
- Prefer concrete types first.
- Define interfaces at the consumer side only when needed.
- Do not panic in normal application flow.
- Run `go test ./...` for code changes when dependencies are available; explain if skipped.

## Naming

- Use code-as-comment naming.
- Names should explain intent clearly enough that extra comments are usually unnecessary.
- Avoid vague names like `handle`, `process`, `data`, `manager`, and `helper`.
- Use complete domain names; abbreviate only standard initialisms like `ID`, `HTTP`, `URL`, `SQL`, `API`, `SSE`, and `JSON`.
