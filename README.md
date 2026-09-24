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
 └── TripBranch  (splitFrom / mergeTo Destination)
      └── BranchDestination (order) → Destination → Location?
Leg  (A→B, vehicle, polyline, Redis cache)
Travel (frozen Leg snapshot when the trip locks)
```

- **authentication** — register, login, refresh, profile, forget / reset password
- **maps** — Goong v2 playground: autocomplete, place detail, geocode, directions, trip TSP
- **trip** — trip CRUD, membership, invite links, and `PUT /trips/:tripId/graph` to replace the route graph while the trip is planning

Goong stands in for Google Maps Platform in Vietnam. Directions default to `bike`. Goong Trip is a single-vehicle TSP, not the product branch graph.

### Trip graph input: `GraphBranch`

The client sends the graph through `PUT /trips/:tripId/graph` as destination ids (`SetTripGraphRequest.Branches`, a `[][]uuid`) plus an `openTail` flag per branch. The service resolves those ids to `Destination` rows and `BuildTripBranches` (`module/trip/model/GraphBranches.go`) turns the resolved `GraphBranch` (`[][]Destination`) into `TripBranch` + `BranchDestination` rows. The response is a `TripDetailResponse`: the trip plus its branches with the audit columns dropped.

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
│       └── location.go      # `seeder location`: flags and the run report
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

**server** listens on `HTTP_ADDR` (default `:8080`). It migrates when Postgres is configured. Without `DB_HOST`, `DB_USER`, `DB_NAME`, and `DB_PORT` the process still serves, and auth has no database. Redis is the cache and password-reset store. OpenAPI UI is `/docs/public`. Swagger comments are on `cmd/server/main.go` (`go generate` in the root `main.go`).

**migrate** connects to Postgres and runs `AutoMigrate`. It does not start HTTP or call Goong.

**seeder** is the parent for reference data. It requires Postgres, migrates, opens Redis as the place cache, then runs one entity. The only entity today is **location**.

`seeder location` reverse-geocodes pins with Goong (`GOONG_MAP_CALC_API_KEY`) and inserts verified locations that are not already in Postgres. A place found only in Redis is written to Postgres. Each pin requests up to `--limit` results (default 10). With no pins, it uses `10.7725,106.6980` and `10.8721512,106.803008`.

Pass pins as `--lat` and `--lng` together, as `--coords` (`lat,lng` pairs separated by `;`), or as bare numbers. `RepairArgs` runs before cobra so a PowerShell-split decimal (`10` `.7486`) is joined back into `10.7486`.

A new seed entity is a file under `cmd/cli/seed/`, registered from `NewSeederCommand`. Put the Goong and persistence work in `internal/seed/<entity>/`.

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
```

Standalone mains. The CLI entry starts at the entity, so it does not take the word `seeder`:

```bash
go run ./cmd/server
go run ./cmd/migrate
go run ./cmd/cli location --lat 10.7486 --lng 106.6601
```

## API

Interactive docs: [http://localhost:8080/docs/public](http://localhost:8080/docs/public)

Goong playground routes need `X-Playground-Key` (`GOONG_PLAYGROUND_KEY`).
