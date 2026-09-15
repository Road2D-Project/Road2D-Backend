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
