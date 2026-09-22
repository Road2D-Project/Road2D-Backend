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
- **trip** — graph models + AutoMigrate; HTTP CRUD is not wired yet

Goong stands in for Google Maps Platform in Vietnam. Directions default to `bike`. Goong Trip is a single-vehicle TSP, not the product branch graph.

### Trip graph input: `GraphBranch`

`GraphBranch` (`module/trip/model/GraphBranches.go`) is the DTO the client sends to describe a trip's route graph. It is a `[][]Destination`: each inner slice is one branch, listed in travel order. `BuildTripBranches` turns it into `[]TripBranch` + `BranchDestination` rows.

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

> **Consideration:** `mergeTo` may become optional. A sub branch that ends without rejoining the graph (a one-way detour, e.g. a scout run or a rider dropping off) currently fails validation; allowing `mergeTo == nil` would keep the split rule strict while letting the tail end open. Not decided yet.

## Run locally

Go 1.26+, PostgreSQL, Redis.

```bash
cp .env.example .env
go run .
```

Default `HTTP_ADDR=:8080`. Auth needs Postgres (`DB_HOST`, `DB_USER`, `DB_NAME`, `DB_PORT`). Redis is used for cache and password-reset tokens. See `.env.example` for the rest. Do not commit `.env`.

```bash
go test ./...
```

## API

Interactive docs: [http://localhost:8080/docs/public](http://localhost:8080/docs/public)

Goong playground routes need `X-Playground-Key` (`GOONG_PLAYGROUND_KEY`).
