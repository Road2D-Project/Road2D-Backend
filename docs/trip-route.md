# Planning, route, and trip

How a planned ride becomes a route. Graph rules (main branch, split, merge, `openTail`) stay in the [README](../README.md#trip-graph-input-graphbranch). This page is the technical half: services, the worker pool, the four models, and the caches.

Covered work starts at `88a3f9c`.

## Contents

- [How a ride is planned](#how-a-ride-is-planned)
- [Location and destination](#location-and-destination)
- [Leg and travel](#leg-and-travel)
- [Services](#services)
- [Worker pool](#worker-pool)
- [Storage tiers](#storage-tiers)
- [Preview one branch](#preview-one-branch)
- [Preview locations before a trip](#preview-locations-before-a-trip)
- [Compute the saved graph](#compute-the-saved-graph)
- [Read the stored travels](#read-the-stored-travels)
- [Draft inbox](#draft-inbox)
- [Not built yet](#not-built-yet)

## How a ride is planned

```mermaid
flowchart TD
  create[Create trip and its draft branch] --> fork[Fork a location into a destination]
  fork --> draft["POST /trips/:id/draft"]
  draft --> graph["PUT /trips/:id/graph"]
  graph --> preview["POST /trips/:id/compute"]
  preview --> look[Polyline only, trip unchanged]
  graph --> lock[Lock the trip]
  lock --> later[ComputeTrip reuses a frozen Travel]
```

1. Create the trip and invite members. The same transaction inserts one draft branch (`IsDraft`, label `Draft`). That branch is an inbox. It is not a ride.
2. Fork a verified `Location` into a `Destination`. The pin keeps its own lat/lng. `POST /trips/:tripId/draft` parks that pin on the inbox. The route graph does not change.
3. The leader calls `PUT /trips/:tripId/graph` while the trip is `planning`. The body is destination ids per branch, plus `openTail`. That replace deletes only route branches (`is_draft = false`) and leaves the inbox. Any active member reads the route back with `GET /trips/:tripId/graph`. Each route branch has its stops and `travels`: `travels[i]` is the stored hop from `stops[i]` to `stops[i+1]`, or null when that hop has not been computed. A locked trip stays readable. `GET /trips/:tripId/travels` returns those same hops without the stop list. The draft branch is absent from both responses.
4. Any active member calls `POST /trips/:tripId/compute` to preview one branch. That branch does not have to match the saved graph, and nothing is written onto the trip.
5. The leader calls `POST /trips/:tripId/travels` while the trip is still `planning`. That routes the saved route branches and upserts `travels`. The draft branch is left out. `TripLocked` exists on the status enum. Nothing sets `Travel.IsFrozen` yet, so a locked trip cannot be recomputed and there is no freeze step.

Goong Trip is not used. That API is a single-vehicle TSP: it needs at least 10 points and it reorders stops. A branch already has an order, so each hop is one Directions call, vehicle `bike`.

## Location and destination

A `Location` is a verified place in the shared catalog (`place_id`, address, lat/lng). Seeding and place detail write it. Planning never updates it.

A `Destination` is a pin. `PlaceService.ForkLocation` copies the location's name and coordinates onto a new pin, sets `LocationID`, and leaves the pin in `editing`. After the link is set, the pin's coordinates are a copy and are no longer editable. A destination has no `TripID` and no `GroupID`. The shared catalog is still `Location`. Membership in a trip is the branch that holds the pin.

The saved route stores destination ids only, through a route `TripBranch` and ordered `BranchDestination` rows. A location never sits on that graph. The preview endpoint is the one place that accepts either id, because a member may try a catalog place that has not been forked yet.

Pins that are not on the ride yet sit on the trip's draft branch. `GET /trips/:tripId/draft` is how the UI lists them. Compute, the route graph, and stored travels never read that branch.

```text
Location (catalog, shared)
    │ fork copies lat/lng
    ▼
Destination (pin, no trip id)
    │ ordered by BranchDestination
    ├── draft TripBranch (IsDraft, hidden inbox) → Trip
    └── route TripBranch (split / merge) → Trip
```

`DELETE /planning/destination/:destinationId` follows where the pin sits. Only draft branches: delete the row, and the inbox stops cascade. Any route branch: the trip must still be `planning`, the stop is stripped, and the route is checked again. A locked trip that still has the pin on a route returns 409 and nothing is written. Disconnecting the route also writes nothing.

## Leg and travel

A `Leg` is the road between two coordinates for one vehicle. It does not know which pin or which trip asked for it. Two pins dropped on the same spot share one leg. The key is `(from_lat, from_lng, to_lat, to_lng, vehicle)`, index `idx_leg_cache`. The row holds the polyline, distance, duration, and turn steps.

A `Travel` is one hop of one trip, between two destination ids. The key is `(trip_id, from_destination_id, to_destination_id, vehicle)`, index `idx_travel_pair`. Trip A freezing A→B does not block trip B from storing its own A→B. `LegID` is set only when the leg already has a Postgres id. A leg that lived only in Redis leaves `LegID` empty. `IsFrozen` stays false while computing. A frozen row is the snapshot kept for a shared history, and it never expires.

`TravelGraph` is `[][]Travel`, one inner slice per branch, in stop order. A branch shorter than two stops keeps an empty slice so branch indexes stay aligned.

## Services

Handlers construct a service per request. The interfaces below are declared in the consumer package, and each one lists only the methods that consumer calls.

| Service | File | Does |
| --- | --- | --- |
| `TripBranchService` | `module/trip/service/TripBranchService.go` | Resolves destination ids, checks branch rules, replaces route branches, and lists or appends the draft inbox |
| `PlaceService` | `module/trip/service/PlaceService.go` | Reads a location or destination, forks a location into a pin |
| `ComputeTripService` | `module/trip/service/ComputeTripService.go` | `PreviewBranch` for one ad-hoc branch, `ComputeTrip` for a saved graph |
| `DirectionService` | `module/maps/service/DirectionService.go` | One A→B route: cache, then Goong, then cache again |

`ComputeTripService` depends on four narrow interfaces:

- `LegRouter` is `DirectionService.Route`.
- `TravelStore` is `TravelRepository` (`FindTravelsByTrip`, `UpsertTravels`). `PreviewBranch` never calls it.
- `DestinationPointFinder` and `LocationPointFinder` load one pin or one place. Only `PreviewBranch` uses them.

`DirectionService` depends on `LegStore` (`Find`, `Upsert`). Which store is passed decides whether a write reaches Redis, Postgres, or neither. The service itself does not know.

`maps/service.TripService.Optimize` is the Goong TSP playground. It is not this trip.

## Worker pool

Both `ComputeTrip` and `PreviewBranch` fan out with the same pool. `computeWorkers` is 5, the same burst as the Goong limiter (`freeRateLimit` on `GoongClient`). Every Directions call waits on that limiter before the HTTP request, so a goroutine past the fifth does not add throughput. The running count is `min(5, number of jobs)`.

```mermaid
flowchart LR
  jobs[Jobs channel] --> w1[Worker]
  jobs --> w2[Worker]
  jobs --> wN[Up to 5]
  w1 --> results[Results channel]
  w2 --> results
  wN --> results
  results --> slots[Write back by index]
```

The steps in `routeJobs` and `previewRoutes`:

1. `context.WithCancel` wraps the request context.
2. `jobs` and `results` are buffered to the job count, so a worker can finish without waiting for the collector.
3. Workers start and range over `jobs`. The caller then sends every job and closes `jobs`. A closed, drained channel is what ends the worker loop.
4. A side goroutine waits for the workers and then closes `results`. The collector ranges until that close.
5. Each result carries the index it came from. Completion order is not branch order, so the collector writes `result[branch][leg]` (or `legs[i]` for a preview).

On the first error a worker sends on `errCh` (buffer 1, so the send does not block) and cancels the context. `limiter.Wait` and the HTTP call in flight then return. Other workers see `ctx.Err()`, skip `Route`, and still drain `jobs` so the closed channel can finish. They do not report that cancel as a second error. The caller returns the first error and drops the partial graph.

`ComputeTrip` dedupes before the pool. The job key is `lat,lng|lat,lng|bike`. One Goong call fills every slot that shares those coordinates, and each slot still stores its own destination ids. That split matters: the travel key is the pin ids, not the coordinates.

`PreviewBranch` does not dedupe. Each consecutive pair is its own job. A repeated coordinate pair becomes two `Route` calls, and the second is a cache hit.

`Route` is called with `Alternatives: false`. `true` would skip the leg cache inside `DirectionService`.

## Storage tiers

```text
PreviewBranch
  shortTTLLegStore  →  Redis only, TTLSeconds = 900
  no Redis          →  noop store, Goong result is not kept

DirectionService on the playground, and ComputeTrip
  CachedLegStore
    1. Redis          key leg:lat,lng|lat,lng|vehicle
    2. Postgres legs  idx_leg_cache, expired row counts as a miss
    3. Goong
  write order: Postgres first, then Redis, so the cached payload has the row id

ComputeTrip, after the legs exist
  Postgres travels   idx_travel_pair, TRAVEL_TTL_SECONDS default 86400
```

`LegStore.Find` returns `nil, nil` for a miss. That is not an error. `DirectionService` treats it as "call Goong".

Redis expiry is the TTL of the key. `LEG_CACHE_TTL_SECONDS` defaults to 86400. If the leg's own `TTLSeconds` is greater than 0, `LocationLegMemoryStore.Upsert` uses that instead. Postgres rows are not deleted when they age out. `LegRepository.Find` treats an expired row as a miss and leaves it, so the next upsert hits `idx_leg_cache` and overwrites it. `Leg.Expired` uses the row's `TTLSeconds` when that is positive, otherwise the caller's fallback window.

`CachedLegStore.Find` reads Redis first. A Redis error is logged and the lookup continues to Postgres. A Postgres hit is written back into Redis. `Upsert` writes Postgres first. The driver returns the row id, and only then is the same struct stored in Redis.

The preview wrapper sits in front of the same Redis store and sets `TTLSeconds = 900` on write. `DirectionService` upserts only after a miss, so a key that is still alive keeps the TTL it already has. A preview miss and a durable miss share the key shape `leg:…`. A short preview entry can therefore satisfy a later durable read until those 15 minutes run out. A durable entry that is already in Redis is not shortened by a preview.

`ComputeTrip` also reads `travels` before it calls `Route`. A frozen row is reused and is not written again. An unfrozen row inside `TRAVEL_TTL_SECONDS` is reused the same way. Anything else is computed, then upserted. The conflict update skips `is_frozen = true`. Rows are deduped by the travel key before the batch, because Postgres rejects an `ON CONFLICT` batch that touches the same key twice.

A missing Redis on the preview path uses `noopLegStore`: `Find` misses, `Upsert` does nothing, and the handler still returns the Goong result.

Two `GoongClient` values exist, one on the map playground and one on `TripController`. Each has its own 5 req/s limiter. They do not add up to a single shared cap.

## Preview one branch

`POST /v1/trips/{tripId}/compute`

JWT, and an active member. The trip does not have to be `planning`. `tripId` is only the membership check.

```json
{
  "points": [
    { "destinationId": "…" },
    { "locationId": "…" },
    { "destinationId": "…" }
  ]
}
```

Each point carries exactly one id. Neither id, or both, is 400 (`ErrInvalidComputePoint`). Fewer than two points is not a hop. An unknown id is 404.

`PreviewBranch` loads each point, builds consecutive pairs, and runs the pool. The response `legs` carry the name and id of each end, `vehicle`, `polyline`, `distanceM`, and `durationS`. There is no `tripId` and no `isFrozen`.

Goong errors: missing API key is 500, rate limit is 429, an empty route or a non-OK status is 502.

## Preview locations before a trip

`POST /v1/planning/preview`

JWT. No trip id and no membership check. The body is ordered catalog location ids, at least two. Each id is a verified place. An unknown id is 404. The call uses the same preview pool and the 15 minute Redis leg cache. Nothing is written to a trip.

```json
{
  "locationIds": ["…", "…"]
}
```

The response is `locations` in that order (`locationId`, `name`, `lat`, `lng`) and `legs` in the same shape as a branch preview. Only `locationId` is set on each end.

That response is the optional `mainBranch` of `POST /v1/trips`. When it is present, create forks each location into a destination, in order, and stores that sequence as the trip's initial main branch. The reviewed hops are stored as travels between those new pins. The catalog coordinates are copied from the location rows; names and coordinates in the payload are not the ones written. The legs must line up with consecutive location ids, use vehicle `bike`, and carry a polyline. A trip created without `mainBranch` still opens with only the draft inbox.

## Compute the saved graph

`POST /v1/trips/{tripId}/travels`

JWT, and the trip leader. No body. The trip must be `planning`; a locked trip returns 400 and is not recomputed.

`ComputeStoredTrip` loads the branches with `FindTripWithBranches`, rebuilds a `GraphBranch` via `GraphFromTrip` (stops already ordered, a missing destination is an error), then calls `ComputeTrip`. `GraphFromTrip` skips every branch with `IsDraft`. A draft pin never becomes a hop. The leg store on this path is `CachedLegStore`: Redis, then Postgres, then Goong. It is not the 15 minute preview cache.

The response is `branches`, one slice per saved branch. Each hop has the destination ids, optional `legId`, vehicle, polyline, distance, duration, `isFrozen`, and `lastComputedAt`. Reused rows keep the database id and the frozen flag. Fresh rows come from `travelFromLeg`: pin ids from the slot, metrics from the leg, `IsFrozen` false.

## Read the stored travels

`GET /v1/trips/{tripId}/graph` and `GET /v1/trips/{tripId}/travels`

JWT, and an active member. Neither call routes or writes. A locked trip is readable.

`GET .../graph` is the screen payload: the trip, each branch's stops, and `travels` aligned to those stops. `travels[i]` is null when that pair has no stored row.

`GET .../travels` is only the hops, one slice per branch, the same shape as the compute response. A hop that has not been computed keeps `fromDestinationId` and `toDestinationId` and an empty `id`. An expired unfrozen row is still returned; expiry only lets a later compute replace it. Rows whose pair is no longer on the saved graph are left out.

Both reads use the same filter as compute. The draft branch is not a slice in either payload, so travel indexes stay aligned with the route branches that remain.

## Draft inbox

`GET /v1/trips/{tripId}/draft` and `POST /v1/trips/{tripId}/draft`

The draft branch is created with the trip. `IsDraft` is the flag. The label `Draft` is only what the row displays. One trip has one inbox.

`GET` returns `{ "destinations": [ ... ] }` in stop order: `destinationId`, `name`, `lat`, `lng`, `orderInBranch`. Any active member may call it, including after the trip leaves `planning`. A trip with no draft row returns an empty list.

`POST` body is `{ "destinationId": "..." }`. Any active member may call it while the trip is `planning`. The pin must already exist. The call appends it and rewrites that one branch. Split and merge stay empty. A second copy of the same pin on the same inbox is rejected, because `idx_branch_dest` is unique per branch. This does not change the route graph or the stored travels.

`PATCH` and `DELETE` on `/trips/{tripId}/branches/{branchId}/stops/{destinationId}` refuse a draft branch id. Removing an inbox pin is `DELETE /planning/destination/{destinationId}`.

## Not built yet

- Nothing sets `IsFrozen` to true.
- Replacing the graph does not delete old travels.
- Preview does not check that a point belongs to the trip's saved graph.
- Friend graph. Trip create and invite still take raw user ids. See [trip membership](trip-members.md).
- Group chat. A trip should be created with one chat whose members match the trip. That object is not stored yet. See [trip membership](trip-members.md).
