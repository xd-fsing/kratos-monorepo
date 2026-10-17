# Kratos 微服务大仓

A Kratos v3 microservices monorepo template. All services share the `api/`
proto contracts and `pkg/` libraries, and deploy independently with their own
ports and databases. The todo service calls the user service over gRPC as the
reference pattern for cross-service calls.

## Services

| Service | HTTP | gRPC | Database | Scope |
|---------|------|------|----------|-------|
| todo | 8000 | 9000 | test_todo | Todo CRUD plus Watch/Sync streams; verifies the owning user over gRPC on create |
| user | 8001 | 9001 | test_user | User Get/Create/List |

## Layout

```
api/<domain>/<version>/     Shared proto contracts for every service.
apps/<service>/             One deployable service per directory.
  cmd/                      Entrypoint, Wire injector, main.go.
  configs/config.yaml       Runtime config. No secrets.
  internal/conf/            Config proto (per service).
  internal/server/          Thin HTTP/gRPC registration over pkg/server.
  internal/service/         Validation, business rules, orchestration.
  internal/data/            Store interfaces + ent/gRPC implementations,
                            typed errors.
pkg/server/                 Shared HTTP/gRPC constructors and middleware.
openapi/                    Merged OpenAPI document for all services.
```

## Quick Start

Prerequisites: Go 1.25+, MySQL 8 (or override the DSN via environment
variables).

```bash
make init        # installs wire and buf (first time only)

make run-user    # terminal A
make run-todo    # terminal B
```

Verify the cross-service call (todo HTTP -> todo biz -> todo data -> user gRPC):

```bash
# Create a user. The body is the User object directly (the HTTP annotation
# maps it onto CreateUserRequest.user, so it is not wrapped in {"user": ...}).
curl -s -X POST http://127.0.0.1:8001/v1/users/create \
  -H 'Content-Type: application/json' \
  -d '{"name":"alice","email":"alice@example.com"}'
# -> {"id":"...","name":"alice",...}

# Create a todo owned by that user. todo calls the user service to verify
# the user exists and returns 404 if it does not.
curl -s -X POST http://127.0.0.1:8000/v1/todos/create \
  -H 'Content-Type: application/json' \
  -d '{"title":"call alice","user_id":"<id>"}'
# -> {"id":"...","title":"call alice","user_id":"<id>",...}

# With the user service stopped the same call fails with 503 USER_UNAVAILABLE.
```

## Build and Codegen

```bash
make build            # builds bin/<service> for every app in apps/
make build-todo       # builds one service
make all              # regenerate api stubs, conf stubs, ent, wire

make api app=user     # generate api stubs for one service only
make config app=user  # generate conf stubs for one service only

make service app=club # scaffold a new empty service under apps/club
                      # (auto-assigns ports 8000+N / 9000+N and db test_club)
```

## Configuration

Each service has its own `configs/config.yaml`: ports, database DSN, and (for
todo) the user service's gRPC endpoint under `client:`. The DSN defaults are
overridable per service:

| Variable | Service | Default database |
|----------|---------|------------------|
| `TODO_DATABASE_SOURCE` | todo | `test_todo` |
| `USER_DATABASE_SOURCE` | user | `test_user` |
| `REDIS_ADDR` | both | `127.0.0.1:6379` |

`auto_migrate: true` is a development convenience; keep it off in production
and apply schema changes as a separate reviewed step.

## Cross-Service Calls

- A service never touches another service's database. It calls the owning
  service's `api/` contract over gRPC.
- The client wrapper and its `Store` interface both live in the caller's
  `data` layer (`apps/todo/internal/data/user.go`); the orchestration that
  uses it lives in `service` (`apps/todo/internal/service/todo.go`).
- The target address is the `client:` section of the caller's config. Use a
  direct host:port locally; swap for a discovery target (e.g.
  `discovery:///user`) once a registry is in place.

## Adding a Service

Copy the skeleton of an existing app (`apps/todo` is the reference), define
its protos under `api/<domain>/<version>/`, generate in order (`make api` ->
`make config` -> ent generate -> wire), and add it to `SERVICES` in the
Makefile. See the "Add-a-service checklist" in `CLAUDE.md`.
