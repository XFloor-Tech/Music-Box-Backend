# AGENTS.md

## Project Overview

Music-Box-Backend is a Go backend service for the Music Box project. Treat this
repository as a Golang blueprint project: keep the structure simple, explicit,
and easy to extend as features are added.

## Stack

- Go is the primary language.
- Chi is used for HTTP routing.
- Viper is used for configuration and environment variable handling.
- Swaggo is used to generate Swagger/OpenAPI documentation.
- Zap is used for structured server logs.
- PostgreSQL is the primary relational database.

## Project Structure

- `cmd/api/` contains the application entry point and server bootstrap.
- `config/` contains configuration loading and environment handling.
- `internal/server/` contains HTTP server setup, middleware, route
  registration, and request handlers.
- `internal/database/` contains PostgreSQL database lifecycle code and schema
  artifacts.
- `internal/auth/` contains authentication domain logic.
- `internal/user/` contains user domain logic.
- `docs/` contains generated Swagger documentation.
- `pkg/` is reserved for reusable packages that are safe to share across
  internal domains.

## Development Guidelines

- Keep HTTP routing in the server layer and delegate business behavior to
  domain services under `internal/`.
- Prefer small, focused packages with clear ownership.
- Read configuration through Viper instead of accessing environment variables
  directly throughout the codebase.
- Use Zap for application and request logging.
- Keep PostgreSQL access behind service or repository boundaries rather than
  scattering SQL across handlers.
- Keep PostgreSQL column names aligned with the existing schema. This project
  intentionally uses camelCase database columns for camelCase domain/API fields
  such as `"userId"`, `"releaseYear"`, `"createdAt"`, and `"updatedAt"`, so
  quote those identifiers in SQL and schema artifacts.
- Keep public JSON responses wrapped in a consistent envelope:
  - `success` is mandatory and must be `true` for successful requests and
    `false` for failed requests.
  - `status` should describe the outcome, such as `success`, `failure`, or
    route-specific values like `ok`.
  - Response field names must be camel cased, including nested response
    payloads such as `nextCursor`, `createdAt`, and `emailVerified`.
  - `data` contains only successful response payloads and should be omitted on
    errors.
  - `error` is an optional top-level string used only when a request fails.
  - Error metadata such as validation `fields` may be top-level alongside
    `error`; do not put error details inside `data`.
- Update Swagger annotations and regenerate docs when changing public HTTP
  APIs.
- Follow existing Go formatting conventions with `go fmt ./...`.

## Common Commands

- `make run` starts the API with hot reload through Air.
- `make build` builds the API binary.
- `make start` builds and runs the API binary.
- `make test` runs all Go tests.
- `make fmt` runs formatting and vet checks.
- `make swagger` regenerates Swagger docs.

## Notes For Future Agents

- Preserve the blueprint shape as the project grows: entry point, config,
  server, middleware, handlers, domain services, and database code should stay
  separated.
- Keep generated Swagger files in `docs/` and avoid manual edits there unless
  absolutely necessary.
- Avoid broad refactors unless they directly support the requested change.
- Before editing, check the working tree and avoid overwriting user changes.
