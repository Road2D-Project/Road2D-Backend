# Road2D Backend

Go API for **Road2D** — safe ride planning for groups on Vietnam roads.

Consumer maps optimize a single rider on one line. Live-location apps only show dots. Trip planners stop before anyone starts riding. Road2D coordinates a **multi-branch spatial graph**: planned splits (scout, logistics, vehicle-specific paths) versus real off-route deviation, early dead-zone warnings, and GPS sampling that does not drain the phone on a long ride.

## Architecture

```
/v1
├── /auth/user/*     public — accounts (JWT, mail reset)
├── /goong/*         playground — Goong maps (X-Playground-Key)
└── /serp-api/*      scratch Google Maps via SerpApi
```

```text
Trip
 ├── TripBranch draft (IsDraft, hidden inbox of Destination pins)
 └── TripBranch route (splitFrom / mergeTo Destination)
      └── BranchDestination (order) → Destination → Location?
Leg  (A→B by coordinates, Redis then Postgres)
Travel (one trip's snapshot of a leg; frozen rows are history)
```

- **authentication** — register, login, refresh, profile, forget / reset password
- **maps** — Goong v2 playground: autocomplete, place detail, geocode, directions, trip TSP
- **trip** — trip CRUD, membership, and invite links. Membership: [docs/trip-members.md](docs/trip-members.md). Planning and routing: [docs/trip-route.md](docs/trip-route.md)
  - [How a ride is planned](docs/trip-route.md#how-a-ride-is-planned)
  - [Location and destination](docs/trip-route.md#location-and-destination)
  - [Leg and travel](docs/trip-route.md#leg-and-travel)
  - [Services](docs/trip-route.md#services)
  - [Worker pool](docs/trip-route.md#worker-pool)
  - [Storage tiers](docs/trip-route.md#storage-tiers)
  - [Preview one branch](docs/trip-route.md#preview-one-branch)
  - [Preview locations before a trip](docs/trip-route.md#preview-locations-before-a-trip) — `POST /planning/preview`
  - [Compute the saved graph](docs/trip-route.md#compute-the-saved-graph) — `POST /trips/:tripId/travels`
  - [Read the stored travels](docs/trip-route.md#read-the-stored-travels) — `GET /trips/:tripId/graph`, `GET /trips/:tripId/travels`
  - [Draft inbox](docs/trip-route.md#draft-inbox) — `GET` and `POST /trips/:tripId/draft`

Goong stands in for Google Maps Platform in Vietnam. Directions default to `bike`. Goong Trip is a single-vehicle TSP, not the product branch graph.

### Trip graph input: `GraphBranch`

The client sends the graph through `PUT /trips/:tripId/graph` as destination ids (`SetTripGraphRequest.Branches`, a `[][]uuid`) plus an `openTail` flag per branch. `GET /trips/:tripId/graph` returns that route for any active member, including after the trip leaves `planning`. Each route branch lists its stops and the stored travels between them; a missing hop is null. `GET /trips/:tripId/travels` returns only those stored hops and does not recompute. The service resolves those ids to `Destination` rows and `BuildTripBranches` (`module/trip/model/GraphBranches.go`) turns the resolved `GraphBranch` (`[][]Destination`) into `TripBranch` + `BranchDestination` rows. The write response is a `TripDetailResponse`: the trip plus its route branches with the audit columns dropped.

Creating a trip also inserts one **draft** branch (`IsDraft`, label `Draft`). It is an inbox for pins that are not on the ride yet. Compute, `GET .../graph`, and `GET .../travels` skip it. Replacing the route graph does not delete it. `GET /trips/:tripId/draft` lists those pins; `POST /trips/:tripId/draft` appends one that already exists. Deleting a destination that sits only on draft branches removes the row immediately. A destination that also sits on a route branch still goes through the route-graph checks.

Rules:

- `graph[0]` is the **main** branch. It has no `splitFrom` / `mergeTo`.
- For every branch, `branch[0]` and `branch[len-1]` are its endpoints.
- Every `graph[n>0]` is a **sub** branch: `branch[0]` is the **split** point and `branch[len-1]` is the **merge** point. Both must appear in at least one other branch, otherwise the sub branch is disconnected and the input is rejected.
- Destinations may **overlap** across branches (a stop can appear in main and in a sub). Only the endpoints carry split / merge meaning.
- A destination must not appear twice within the same branch (`idx_branch_dest` is unique per branch).
- Order of stops inside a branch is the slice index (`OrderInBranch`), including the split / merge stops.

```text
graph = [
  [d0, d1, d2, d4, d5, d9],   // main
  [d1, d3, d9],               // sub: split at d1, merge at d9
  [d0, d4, d6, d7, d8, d9],   // sub: split at d0, merge at d9, overlaps main at d4
]
```

Validation is deliberately loose: the backend only checks that endpoints connect to the graph. It does not verify geographic plausibility or that a split point precedes its merge point on the parent branch — the client owns that.

`openTail[i] = true` makes the merge optional for sub branch `i`: the branch still splits at its first stop but ends without rejoining, so `mergeTo` stays nil and its last stop does not have to appear anywhere else. `openTail[0]` is always false, since the main branch has no merge. This covers a one-way detour such as a scout run or a rider dropping off. The graph can only be replaced while the trip is `planning`.

## Process layout

`main.go` is the unified binary. It calls `cmd.Execute()`, which dispatches `server`, `seeder`, and `migrate`. Each of those can also be started from its own `main` under `cmd/`. `internal/` is seeder domain logic: no HTTP, no cobra.

```text
main.go                      # cobra root: server | seeder | migrate
cmd/
├── root.go                  # load .env, repair PowerShell args, dispatch
├── server/
│   ├── main.go              # standalone API; swag comments live here
│   └── run/run.go           # Postgres, Redis, Gin, /v1, /docs/public
├── cli/
│   ├── main.go              # standalone seeder; args start at the entity
│   └── seed/
│       ├── command.go       # `seeder`: Postgres + Redis, then AutoMigrate
│       ├── location.go      # `seeder location`: flags and the run report
│       └── trip.go          # `seeder trip`: members, trip, and a main branch from location ids
└── migrate/
    ├── main.go              # standalone migrate
    └── run/run.go           # Postgres, then AutoMigrate
internal/
└── seed/location/
    ├── seeder.go            # geocode pins, skip known places, insert rows
    ├── map.go               # Goong place detail → verified Location
    ├── coords.go            # parse lat/lng tokens; fallback pins
    └── args.go              # stitch decimals PowerShell splits apart
```

**server** listens on `HTTP_ADDR` (default `:8080`). `PORT` is accepted the same way when `HTTP_ADDR` is empty. Without `DB_HOST`, `DB_USER`, `DB_NAME`, and `DB_PORT` the process still serves, and auth has no database. Redis is the cache and password-reset store. `GET /health` pings Postgres and Redis and returns 200 only when both answer. OpenAPI UI is `/docs/public`. Swagger comments are on `cmd/server/main.go` (`go generate` in the root `main.go`).

**migrate** connects to Postgres and runs `AutoMigrate`. It does not start HTTP or call Goong. Schema rules are in [Migrations](#migrations).

**seeder** is the parent for reference data. It requires Postgres, migrates, opens Redis as the place cache, then runs one entity. Entities today are **location**, **user**, and **trip**.

`seeder location` reverse-geocodes pins with Goong (`GOONG_MAP_CALC_API_KEY`) and inserts verified locations that are not already in Postgres. A place found only in Redis is written to Postgres. Each pin requests up to `--limit` results (default 10). With no pins, it uses `10.7725,106.6980` and `10.8721512,106.803008`.

Pass pins as `--lat` and `--lng` together, as `--coords` (`lat,lng` pairs separated by `;`), or as bare numbers. `RepairArgs` runs before cobra so a PowerShell-split decimal (`10` `.7486`) is joined back into `10.7486`.

`seeder user --default` registers `USER_NAME` when that username is missing, then logs in with `USER_PASSWORD` and prints the access and refresh tokens. Set `USER_NAME`, `USER_EMAIL`, and `USER_PASSWORD` in `.env`.

`seeder user --fake` registers every account in `internal/seed/user/users.json` (five sample riders; the file is a JSON array and is not capped). Usernames already in Postgres are skipped. `--file path.json` reads another array of `{username, email, password}` instead.

`seeder trip` creates a planning trip owned by `USER_NAME` (registered first when that username is missing). It does not create a group. With no `--members`, every account in `internal/seed/user/users.json` except the leader is seated, and missing accounts are registered first. `--members` names a shorter list. The main branch is forked from `--location-ids` in that order; with no flag, the earliest 10 locations are used. The branch is the destination ids returned by those forks. The command prints the trip without secrets, plus the join token. Only `bronze` (15 seats) can be created until the other trip types have a member cap. Run `seeder location` first so there are places to copy.

A new seed entity is a file under `cmd/cli/seed/`, registered from `NewSeederCommand`. Put the Goong and persistence work in `internal/seed/<entity>/`.

## Migrations

`go run . migrate` calls `configuration.AutoMigrate`. That function does two things, in order:

1. `version.Apply` runs every step in [module/share/configuration/version](module/share/configuration/version/Steps.go), oldest first.
2. GORM `AutoMigrate` creates missing tables and adds missing columns and indexes from the model structs.

There is no `schema_migrations` table. Each step runs on every migrate, including a database that is already current. A fresh database usually has no table yet, so the step returns immediately and `AutoMigrate` builds the column from the model.

GORM will not drop a column, change a column type, narrow a unique index, or clear `NOT NULL`. Those edits are a version step, and the step runs before `AutoMigrate`. When a new column must be `NOT NULL`, backfill existing rows in that step first (`V4_AddTripPolicyColumns` does this).

Rules for a new step:

- Add the next number. The shipped steps are `V1` through `V6`. Do not edit a step that has already run on a shared database; add `V7_...` instead.
- Name the file and the function the same: `V<n>_<WhatItDoes>.go` and `func V<n>_<WhatItDoes>(db *gorm.DB) error`.
- Register it at the end of `Steps` in `Steps.go`. Order is the number, not the file list on disk.
- Make the SQL safe to run twice. Missing table, missing column, or the new shape already present means return nil.
- Put the desired end state on the GORM model as well. The version step only repairs databases that already exist. `AutoMigrate` creates that shape on a new database.

## Run locally

Go 1.26+, PostgreSQL, Redis.

Full file environment variable.
Default `HTTP_ADDR=:8080`. Auth needs Postgres (`DB_HOST`, `DB_USER`, `DB_NAME`, `DB_PORT`). Redis is used for cache and password-reset tokens. See `.env.example` for the rest. Do not commit `.env`.

```bash
cp .env.example .env
go test ./...
```

Unified binary (`go run .` or `go build`, which writes `Road-To-Destination-BE.exe` on Windows):

```bash
go run . server
go run . migrate
go run . seeder location --lat 10.7486 --lng 106.6601
go run . seeder location --coords "10.7486,106.6601;10.7725,106.6980"
go run . seeder user --default
go run . seeder user --fake
go run . seeder trip
```

Standalone mains. The CLI entry starts at the entity, so it does not take the word `seeder`:

```bash
go run ./cmd/server
go run ./cmd/migrate
go run ./cmd/cli location --lat 10.7486 --lng 106.6601
```

## Run with Docker

Copy `.env.example` to `.env` and set `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and the three JWT secrets. Compose reads that file. The app container overrides `DB_HOST` to `postgres` and `REDIS_ADDR` to `redis:6379`.

```bash
docker compose up --build
```

The API is [http://localhost:8080](http://localhost:8080). `GET /health` is the process check; Compose marks `r2d-backend` healthy only when that body has `"status":"ok"`. Docs stay at [http://localhost:8080/docs/public](http://localhost:8080/docs/public).

The app image is Alpine, so the shell is `sh`. `seeder` is a subcommand of the same binary that runs the server:

```bash
docker exec -it r2d-backend sh
./Road-To-Destination-BE seeder user --default
./Road-To-Destination-BE seeder location --lat 10.7725 --lng 106.6980
./Road-To-Destination-BE seeder trip
```

From the host, without opening a shell:

```bash
docker exec -it r2d-backend ./Road-To-Destination-BE seeder user --fake
```

The Compose project name defaults to the directory (`road-to-destination-be`). Set `name:` at the top of `docker-compose.yml`, or `COMPOSE_PROJECT_NAME` in `.env`, or run `docker compose -p r2d up`. Container names are already `r2d-backend`, `r2d_db`, and `r2d_redis`.

## API

Interactive docs: [http://localhost:8080/docs/public](http://localhost:8080/docs/public)

Goong playground routes need `X-Playground-Key` (`GOONG_PLAYGROUND_KEY`).
