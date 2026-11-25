# Tracking checkpoints and Redis Streams

The tracking server owns live room state. Redis Streams distributes redirect
calculations to separate workers. PostgreSQL stores recovery checkpoints and
completed redirect results. The WebSocket protocol is described in
[realtime rooms](realtime.md); the off-route rules are in
[tracking deviation](tracking-deviation.md).

## Responsibilities and data flow

`TrackingCoordinator` applies GPS samples, snapping results, lives, stay mode,
and redirect results on the room goroutine. Workers calculate routes and return
results; they do not update the coordinator's state directly.

```text
WebSocket GPS -> TrackingCoordinator -> WebSocket broadcasts / snapshots
                      |
                      +-> CheckpointWriter -> PostgreSQL tracking_checkpoints
                      |
                      +-> RedirectPublisher -> Redis jobs stream
                                                  |
                                             RedirectWorker -> Goong
                                                  |
                                      PostgreSQL tracking_redirect_results
                                                  |
                                           Redis results:<ownerId>
                                                  |
                                      RedirectResultConsumer -> room inbox
                                                  |
                                       coordinator applies result on tick
```

`CheckpointWriter`, `RedirectPublisher`, and `RedirectResultConsumer` run inside
the API server process. `RedirectWorker` runs in a separate `worker` process.
Multiple workers share the job consumer group. The current deployment assumes
one tracking server owns the live trips. Multiple tracking servers still need
trip ownership and connection routing.

## Checkpoint timing and recovery

- `TRACKING_CHECKPOINT_INTERVAL_SECONDS` defaults to `5` and must be positive.
  A tick offers a checkpoint only when room state is dirty and the interval has
  elapsed. The first dirty checkpoint can be offered immediately.
- Disconnect, trip stay transitions, completion, and room close request an
  earlier checkpoint. Offering a checkpoint queues it in memory; it does not
  mean PostgreSQL has committed it yet.
- `CheckpointWriter` keeps the latest pending snapshot per trip. It retries
  failed writes and uses a three-second timeout for each database write.
- The writer assigns an increasing `sequence` per trip, starting above the
  restored sequence. The PostgreSQL upsert replaces a row only when its
  sequence is newer. `revision` counts room state changes; neither number is a
  Redis stream ID or the number of GPS messages durably recorded.

These are periodic snapshots, not a transactional outbox or an event log.
A crash can lose changes since the last successful save. The configured interval
is a target cadence, not a maximum data-loss window during database outages.
The publisher queue is also in memory; saving a checkpoint and publishing a job
are separate operations.

After a server restart, the first authorized join loads the trip and its saved
checkpoint. Recovered riders start disconnected with
`awaitingFreshPosition: true`. Their stored coordinates retain `observedAt`.
The coordinator sends `tracking.snapshot` on join and on
`{"type":"tracking.resync","payload":{}}`.

Fresh GPS clears the waiting flag before snapping, deviation counting, and
redirect recovery resume. Saved branch/hop progress is restored when compatible
with the current assigned branch. Private resting coordinates and redirect
details are visible only to that rider in the snapshot.

## Stream keys and payloads

- **Jobs:** `road2d:tracking:redirect:jobs`, group `redirect-workers`.
  Each entry has a `job` field containing JSON with `jobId`, `ownerId`, `tripId`,
  `memberId`, `generation`, `schemaVersion`, `origin`, `target`, and `expiresAt`.
- **Results:** `road2d:tracking:redirect:results:<ownerId>`, group `tracking-owner`.
  Each entry has a `result` field containing JSON with the same job identity,
  route geometry and metrics, or an `error`.
- **Dead letter:** `road2d:tracking:dead`. Invalid entries are copied here with
  `source`, `entryId`, `error`, and the original `payload`, then acknowledged
  and removed from their source stream. Its approximate maximum length is 10,000.

`ownerId` is a new UUID for each server process. It is not a trip/room UUID.
Results for different rooms on that server share the same owner stream.
The server and workers refresh result-stream TTLs to 24 hours. An old owner's
stream expires after its last refresh. The jobs and dead-letter streams have
no key TTL.

Startup creates the relevant streams and groups with `XGROUP CREATE ... 0
MKSTREAM`; an existing group is reused. Keep workers running continuously.
Group creation is a startup step; the process must stay alive to handle jobs.
Use Redis 6.2 or newer, or a compatible cloud service that supports `XAUTOCLAIM`,
consumer-group reads, expiry, and Lua scripting.

## Delivery, retries, and acknowledgement

1. The coordinator broadcasts `tracking.redirect` with `status: pending` and
   offers a job to the publisher's bounded queue. Network I/O happens outside
   the room goroutine. A full queue is retried on later ticks.
2. The publisher uses `XADD`. A worker claims idle pending entries with
   `XAUTOCLAIM`, or reads new ones with `XREADGROUP`.
3. The worker looks up `jobId` in `tracking_redirect_results`. A stored result
   is reused. Otherwise it calls Goong and saves the result in PostgreSQL.
4. It publishes the saved result to the owner's result stream, sets its TTL,
   then atomically runs `XACK` and `XDEL` on the job entry using Lua.
5. The server's result consumer places the result in the room inbox, then
   acknowledges and deletes the result entry. If the inbox is full, the entry
   stays pending for another attempt. Results for rooms without a subscription
   are acknowledged and discarded.
6. On tick, the coordinator checks the trip, member, current job ID, generation,
   and pending state before applying a result and broadcasting `ready` or `failed`.

The acknowledgement in step 5 confirms inbox delivery, not a committed
checkpoint. A crash between delivery and checkpoint persistence is handled by
restoring the checkpoint and resuming from fresh GPS.

Job reclaim uses a 30-second minimum idle time; result reclaim uses one second.
A worker attempt has a 20-second context. The Goong request timeout is 10 seconds,
bounded by job expiry. The coordinator considers a redirect request too old after
30 seconds and follows its provider-error retry policy.

Redelivery of a saved job result avoids another provider call. A crash before
the result is saved can repeat the call, and result publication can be duplicated.
This is not exactly-once execution. A reclaimed expired job without a saved result
produces an error result instead of calling Goong. Normal provider errors are
saved results, not dead-letter entries.

After a server restart, old jobs still target the old owner stream. The new
coordinator recreates pending redirects after fresh GPS with a new job ID; it
does not adopt the dead server's result stream.

## Run against cloud PostgreSQL and Redis

Set the shared connection values in the local `.env` or deployment environment:

```dotenv
DB_URL=postgresql://<user>:<password>@<db-host>:<port>/<database>?sslmode=require&search_path=<schema>
REDIS_URL=rediss://<user>:<password>@<redis-host>:6379/0
TRACKING_CHECKPOINT_INTERVAL_SECONDS=5
GOONG_MAP_CALC_API_KEY=<provider-key>
```

Replace placeholders with provider connection values and URL-encode credentials.
Use the database's intended schema in `search_path`. Both processes must use the
same database/schema, Redis database, and Goong quota configuration. `DB_URL` and
`REDIS_URL` take precedence over the separate host/port variables. `rediss://`
enables TLS. Keep credentials outside committed docs and scenario files.

Apply schema changes once per deployment as needed:

```powershell
go run . migrate
```

Then leave these running in separate terminals:

```powershell
go run . server
```

```powershell
go run . worker
```

No local PostgreSQL or Redis container is needed for cloud connections.
The repository's Docker Compose setup instead starts local PostgreSQL, Redis,
the API, and a worker; it overrides the separate host variables for containers.
`GET /health` checks PostgreSQL and Redis connectivity, not worker liveness or
queue progress.

## Inspect Redis and PostgreSQL

Connect to a TLS Redis endpoint without putting the password in the command:

```powershell
redis-cli --tls -h <redis-host> -p 6379 --user <redis-user> --askpass
```

The following commands also work in a provider's Redis command console:

```redis
XINFO GROUPS road2d:tracking:redirect:jobs
XINFO CONSUMERS road2d:tracking:redirect:jobs redirect-workers
XPENDING road2d:tracking:redirect:jobs redirect-workers
XLEN road2d:tracking:redirect:jobs
XLEN road2d:tracking:dead
SCAN 0 MATCH road2d:tracking:redirect:results:* COUNT 100
```

Repeat `SCAN` with the returned cursor until it returns `0`; `COUNT` is a hint.
Copy an exact result key from that output to inspect it:

```redis
XINFO GROUPS road2d:tracking:redirect:results:<ownerId>
XPENDING road2d:tracking:redirect:results:<ownerId> tracking-owner
TTL road2d:tracking:redirect:results:<ownerId>
```

`pending` counts delivered but unacknowledged entries; group `lag`, when
available, counts entries not yet delivered. `XLEN = 0` is expected after a
successful run because acknowledged entries are deleted. Stream keys, consumer
metadata, and historical counters can remain. A consumer record alone does not
prove that a worker is currently alive. `NOGROUP` or a missing key before startup
means that stream/group has not been created yet.

Inspect checkpoint metadata in SQL using the same schema as the application:

```sql
SHOW search_path;
SELECT trip_id, sequence, saved_at FROM tracking_checkpoints ORDER BY saved_at DESC LIMIT 10;
SELECT count(*) AS saved_redirect_results FROM tracking_redirect_results;
```

In a provider SQL editor, qualify the tables when the session uses another
schema, for example `road_to_destination.tracking_checkpoints`. An empty `public`
schema does not mean the application's tracking tables are missing.

## Clean up after a test

Stop the simulator, API server, and all workers using these test streams first.
The jobs and dead-letter keys are shared across trips; deleting them discards
remaining work for every trip using those keys. Use this reset only where all
that work can be discarded.

```redis
DEL road2d:tracking:redirect:jobs
DEL road2d:tracking:dead
SCAN 0 MATCH road2d:tracking:redirect:results:* COUNT 100
```

Finish the scan, review the returned keys, and delete each selected owner stream
by its exact name:

```redis
DEL road2d:tracking:redirect:results:<ownerId>
```

`DEL` removes the stream and its consumer-group state. It does not accept a
wildcard pattern. Startup recreates the necessary streams/groups. Trimming
entries alone does not provide the same full reset of consumer-group state.
Do not use `FLUSHDB` or `FLUSHALL` on the shared application Redis database.

This reset preserves PostgreSQL checkpoints and saved redirect results, so a
reconnecting user can still receive recovered state. Neither tracking table has
an automatic retention cleanup in this version. Resetting durable room state is
a separate database operation and should be scoped to the intended test trip.

## Verification and current limits

See [the simulator guide](../internal/test-realtime/README.md#kiểm-tra-checkpoint-và-redis-worker)
for the live test procedure. The cloud smoke run verified redirect round-trip,
rejoin, disconnect/reconnect, stay transitions, and checkpoint recovery after an
API server crash. Worker-crash reclamation and reuse of saved results are covered
by worker tests; that smoke run did not kill a worker mid-job or test high load.

Run the recovery and runtime tests from the repository root (race detection
requires a supported C toolchain):

```powershell
go test -race ./module/tracking/... ./module/realtime/... ./module/share/configuration ./cmd/...
```

The simulator Go module has its own tests under `internal/test-realtime/go`.

Access tokens are currently checked at the WebSocket handshake. An established
socket is not closed automatically at token expiry. Reconnect with an expired
token returns HTTP 401, and the simulators do not refresh tokens automatically.

## Code and Redis references

- State and snapshots: `module/tracking/Checkpoint.go`, `Redirect.go`, `OnTick.go`.
- Stream processing: `module/tracking/worker/`.
- Durable writes: `module/tracking/repository/`.
- Process wiring: `cmd/server/run/run.go`, `cmd/worker/run/run.go`.
- Redis command semantics: [XAUTOCLAIM](https://redis.io/docs/latest/commands/xautoclaim/),
  [XACK](https://redis.io/docs/latest/commands/xack/),
  [SCAN](https://redis.io/docs/latest/commands/scan/),
  [DEL](https://redis.io/docs/latest/commands/del/).
