# PostgreSQL outbox

The outbox is a durable list of work to perform after a domain transaction
commits. Message/read/preferences changes and their event are written in the
same PostgreSQL transaction. A failed event insert rolls back the domain change.
Chat dispatch polls PostgreSQL directly; Redis Streams continue to serve the
independent tracking workers.

`module/share/outbox` owns claims, retries and retention. `module/chatting/Delivery.go`
reads current authorized recipients and summaries, then prepares socket frames.
Room callbacks perform no database IO. The server wires both parts in
`cmd/server/run/run.go`.

## Claim query

`Repository.Claim` uses one SQL statement:

```sql
WITH candidates AS (
  SELECT e.id FROM outbox_events e
  WHERE e.dispatched_at IS NULL
    AND e.next_attempt_at <= clock_timestamp()
    AND (e.claimed_until IS NULL OR e.claimed_until <= clock_timestamp())
    AND NOT EXISTS (
      SELECT 1 FROM outbox_events prior
      WHERE prior.conversation_id = e.conversation_id
        AND prior.dispatched_at IS NULL
        AND prior.event_seq < e.event_seq
    )
  ORDER BY e.created_at, e.id
  LIMIT ? FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events e
SET claim_token = ?,
    claimed_until = clock_timestamp() + (? * INTERVAL '1 millisecond'),
    attempts = attempts + 1
FROM candidates WHERE e.id = candidates.id
RETURNING e.*;
```

- `dispatched_at IS NULL`: the event has not completed server-side handoff.
- `next_attempt_at`: retry backoff has elapsed.
- `claimed_until`: no other worker has an unexpired lease.
- `NOT EXISTS prior`: an earlier pending sequence blocks later events in that
  conversation, including when the earlier event is leased or backing off.
  Different conversations can progress independently. Standalone session and
  deleted-conversation events have no conversation sequence or ordering guarantee.
- `FOR UPDATE SKIP LOCKED`: briefly lock selected rows and skip those another
  claim transaction holds. Workers do not wait for each other's row locks.
- `UPDATE ... RETURNING`: atomically record ownership, extend the lease, increment
  the attempt count and return the claimed rows. A separate SELECT then UPDATE
  would leave a race where two workers select the same event.

The database locks end when this statement's transaction commits. Socket work
runs afterward under a time-limited lease, without holding a DB transaction.
The worker claims one event at a time with a 30-second lease. Handoff has a
5-second context and acknowledgement has a separate 2-second context; a serial
batch cannot sit waiting until the leases at its tail expire.

Each claim generates a new random `claim_token`. `Complete` and `Retry` match
both event ID and claim token. If worker A pauses until its lease expires and
worker B reclaims the event, A's old token cannot complete or reschedule B's
claim. Expiry alone does not forbid A from acknowledging if nobody reclaimed it.

## Delivery and retries

The dispatcher drains ready work, polling every 250 ms when none is ready.
Successful handoff sets `dispatched_at` and clears claim metadata. Failure clears
the lease and schedules exponential backoff: 1, 2, 4, …, 256 seconds. There is no
automatic discard after a maximum attempt count; inspect persistent failures.

A crash before handoff leaves a pending event that becomes claimable after its
lease expires. A crash after handoff but before `Complete` can deliver it again.
Delivery is therefore at least once at the server handoff boundary, not exactly
once at the client. Claim-token fencing protects DB acknowledgements, not a
socket write that already happened.

`PublishToRoom` acceptance means queued to an existing room, not received by a
client. Offline rooms are skipped and do not keep events pending. Slow sockets
and disconnects can lose frames. Reconnect clients fetch current REST state and
authorized replay. The outbox does not hold a sleeping user's room in memory.

The delivery worker keeps up to 4096 accepted-purpose keys:
`eventId + conversation/personal + roomId + recipientUserId`. If conversation
handoff succeeds but personal handoff fails, retry can finish personal delivery
without repeating the accepted conversation handoff. The cache disappears on
restart; clients must deduplicate too. Private read/preferences events target
only their recipient. Personal badges contain current absolute summary state,
never message text or a preview.

## Access changes

Roster changes persist an internal event in the same transaction. Trip deletion
persists an independent removal event without a conversation foreign key, so
cascading the conversation cannot erase that follow-up. Former recipients from
pending roster removals are retained in the deletion event.

Login issues access/refresh tokens sharing `sid` and distinct `jti` values.
Refresh preserves `sid`; new access expiry never exceeds refresh expiry.
Logout stores the refresh hash and login ID in PostgreSQL and inserts a session
revocation event atomically. Redis is only a hash cache. Fresh access-token
validation checks the current PostgreSQL session version and login denylist.
Legacy tokens without `sid` use account-version invalidation on logout. Password
reset updates the password, advances that version and inserts its event in one
transaction. A delayed revocation matches only the old login/version.

Admission (before JWT DB validation), access-changing commits/local eviction and
delivery share one context-aware process gate. A removed member cannot pass an
old authorization check and join behind eviction, and a prepared frame cannot
be enqueued behind a local revocation. Domain repositories depend on narrow
lifecycle interfaces, wired to chatting by the server; they do not import sockets.

The HTTP mutation waits for local eviction ACK before success. Request
cancellation cannot skip post-commit revocation. Failed queue/ACK falls back to
room closure with an independent bounded context; retained users can reconnect.
If the current roster cannot be reread after commit, close the whole conversation
room and leave durable follow-up to retry. A failed final closure can return an
error after the DB has committed; clients must reload state before retrying.

This is a single-owner server design. SQL claims can coordinate multiple
claimers, but local room maps, the access gate and the dedup cache are not shared.
Running socket owners on multiple servers needs owner routing or a fanout bus
and distributed revocation coordination; adding competing claimers alone is
insufficient.

## Retention and shutdown

Retention runs independently of rooms. Every minute it deletes dispatched events
older than seven days in transactions of at most 200 rows, repeating until no
eligible rows remain. Pending events are never deleted for age. PostgreSQL can
reuse dead-row space after autovacuum; deleting rows does not immediately shrink
the table file on disk.

Before deleting conversation events, cleanup locks the parent conversation and
advances its durable `replay_floor` in the same transaction. Replay checks this
floor as well as the existing time window. A cursor below the floor returns 410
with resync bounds even after the expired source rows are physically gone.
Private/internal events can leave sequence gaps; clients must use returned
replay cursors rather than infer a contiguous public sequence.

Shutdown stops HTTP admission, stops new claims, drains an accepted handoff while
hubs remain open, closes hubs and stops the remaining background workers.
Unacknowledged rows remain pending for recovery. Tracking's final checkpoint
flush remains part of server shutdown.

Run `go run . migrate` before starting against an existing database. AutoMigrate
adds the lease fields, session ID and replay floor; V13 adds pending-conversation
and dispatched-retention indexes. Useful operational signals are pending count,
age of the oldest pending row, attempt counts and dispatch errors. A failed
earlier event intentionally stalls later events for that conversation until it
recovers; do not manually mark it complete without assessing replay and fanout.
