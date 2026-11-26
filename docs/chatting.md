# Chatting sockets

Phase 1 steps 8–9 add two WebSocket endpoints and durable outbox delivery. `module/conversation` owns REST,
messages, read state and authorization. `module/chatting` coordinates the live
connections. `module/realtime` owns room queues, pumps, expiry and cleanup.
There is no database IO in the room modules or their factories.

The files follow tracking's coordinator/callback structure:

```text
module/chatting/
  ChattingCoordinator.go   # Conversation state and NewChattingModule factory
  PersonalCoordinator.go  # Personal state and NewPersonalModule factory
  ChattingHubs.go          # Creates and mounts the two independent hubs
  OnJoin.go
  OnMessage.go
  OnLeave.go
  OnTick.go
  Close.go
  Typing.go
  Sync.go
  helpers.go
  model/message/          # Socket commands/events, separate from HTTP DTOs
    TypingMessage.go
    TypingEvent.go
    SyncRequired.go
  controller/             # HTTP upgrade handlers and origin policy
```

Message/read/preferences mutations use the existing conversation REST APIs.
Their transaction also inserts an outbox event. After commit, the dispatcher
hands authorized invalidations to the conversation room and current badge state
to the recipient's personal room. Offline rooms are skipped, never created.
See [outbox delivery and recovery](outbox.md) for claims, retries and retention.

## Connect and authenticate

- `GET /v1/realtime/me`: keep open while the app is in use. The room ID is the
  authenticated user's UUID; query parameters cannot choose another user.
- `GET /v1/realtime/conversations/:conversationId`: open for the conversation
  currently displayed. Close it when switching/leaving. The client chooses
  this one-active-conversation convention; the server does not enforce a global
  one-conversation cap per user.

Both endpoints authenticate before upgrading. Native clients use
`Authorization: Bearer <access token>`. Browser WebSockets use
`?access_token=<access token>`; existing `LiftAccessToken` removes it before
application request logging. Use TLS outside localhost and keep upstream access
logs from recording this query. Header authentication takes precedence.

The conversation endpoint calls the same `ConversationService.GetConversation`
as REST before the upgrade, using current PostgreSQL membership. An inaccessible
or nonexistent conversation returns 404. Invited, pending, rejected, left and
kicked memberships do not grant access, even for a discoverable trip. Active
members can connect during planning, locked, active, stay and completed states.

Invalid JWT/session returns 401, malformed conversation UUID returns 400,
disallowed browser origin returns 403, and an unavailable conversation read
returns 503 without internal database details. After an upgrade, failures close
the socket instead of writing HTTP JSON.

`JoinWithOptions` carries the verified access token's `jti`, login `sid`, session version
and expiry into the connection. An expiry timer and read/write deadlines close
it at token expiry, even if pongs keep arriving. Reconnect using a refreshed
access token; there is no token-refresh command on the socket. The `jti` is an
access-token identifier, not a shared login ID with its refresh token.

Logout closes conversation and personal sockets for that login `sid`. Password
reset advances the account session version and closes sockets from older
versions. Legacy credentials without `sid` use account-version revocation on
logout. Refresh preserves the login ID and never extends an access token beyond
its refresh token's expiry. Fresh handshakes and authenticated REST requests
check current session version and the PostgreSQL login denylist.

Kick/leave closes the removed user's conversation socket while preserving `/me`.
Trip deletion closes that conversation. These changes persist a revocation
event in the mutation transaction and wait for local eviction before success.
Admission, delivery and access-changing commits share a context-aware process
gate, acquired before authentication, so an old authorization snapshot cannot
join or publish after local revocation. Failed queued eviction falls back to
closing affected rooms; retained users can reconnect. The deployment remains
single-owner; these in-process hubs do not provide multi-server fanout.

## Configuration

`CHAT_ALLOWED_ORIGINS` is an optional comma-separated list of exact HTTP(S)
frontend origins, for example:

```dotenv
CHAT_ALLOWED_ORIGINS=http://localhost:5173,https://app.example.com
CHAT_ROOM_IDLE_TIMEOUT_SECONDS=60
```

Configured origins must not contain credentials, query strings, arbitrary paths
or wildcards. With no configured list, browser requests must have the same
scheme and host/port as the backend request. Behind a TLS-terminating proxy,
configure the public frontend origin explicitly; forwarded headers do not expand
the allowlist. Native requests without an Origin still require Bearer auth.

Conversation and personal hubs use the configured idle timeout (default 60
seconds), no module tick, 64 control commands and a 16 KiB external event limit.
Tracking keeps its independent hub and defaults. Idle events do not create or
keep offline rooms alive. Each hub still keeps only the newest socket per user
in a room; simultaneous multi-tab/device fanout is not implemented.

## Wire commands and sync hints

The JSON envelope is the existing `{type, payload}` shape. Both channels first
send a targeted sync hint, followed by ordinary room-local presence:

```json
{"type":"chat.syncRequired","payload":{"scope":"conversation","conversationId":"<uuid>"}}
```

Personal joins send `{"scope":"personal"}` instead. A hint carries no replay
watermark, message body or unread count. Fetch the conversation list on a
personal hint; fetch authorized detail/history/replay on a conversation hint.

Both channels accept:

```json
{"type":"chat.resync","payload":{}}
```

It returns another sync hint, without reading the DB in a room callback. Hints
are throttled to one per second per connected user, including the initial join
hint. Repeated requests inside that interval are ignored. Unsupported commands
return `system.error` with `unsupported_command`; invalid JSON shapes return
`invalid_payload`. REST create/edit/delete/read/preferences are not WS commands.

Start buffering live events before fetching REST state. Catch up from the last
applied event cursor, paginate using the cursor returned by the server, then
merge/deduplicate buffered events. Private events can create sequence gaps.
If replay returns 410, clear stale pages and reload current state using the
response's sync bounds. A personal reconnect must reload the conversation list;
personal rooms do not keep a separate durable replay log.

## Typing

Only the conversation channel accepts:

```json
{"type":"chat.typing","payload":{"typing":true}}
```

The server broadcasts the authenticated user ID, conversation ID, boolean and
`expiresAt` to that room, including the sender. Clients must remove an indicator
at that timestamp even if no explicit stop arrives. True updates are limited to
one per second; an explicit false is sent immediately while typing is active.
A true/false toggle cannot immediately bypass the true-update limit. Expiry is
five seconds and uses no periodic room tick. Leaving emits a stop when needed
and removes the user's transient state. Typing is never stored in the outbox.

## Committed events

Conversation rooms receive `chat.messageCreated`, `chat.messageUpdated`,
`chat.messageDeleted`, `chat.readUpdated` and `chat.preferencesUpdated`.
The payload matches REST replay: `eventId`, `type`, `conversationId`, `eventSeq`,
`payload` and `createdTime`. Message payloads identify message IDs, sequences
and revisions; fetch authorized REST detail for content. Read/preferences events
are sent only to their recipient. Internal roster/session events are not exposed
through conversation replay.

`/me` receives `chat.conversationUpdated` with absolute current state:

```json
{"type":"chat.conversationUpdated","payload":{"eventId":"<uuid>","conversationId":"<uuid>","summaryVersion":7,"unreadCount":2,"lastReadSeq":3,"lastMessageSeq":5,"lastMessageId":"<uuid>","lastMessageRevision":1,"lastActivityAt":"2026-11-26T12:00:00Z","activeMemberCount":6,"reason":"chat.messageCreated"}}
```

`mutedUntil` is optional. There is no message preview, body or media URL in this
event, including when `unreadCount` is zero. Render an `x new messages` indicator;
fetch message content through REST when the UI needs it. Counts exclude the
user's own messages and deleted messages, so sequence subtraction is not the
unread-count formula. An event processed after a read returns the latest DB
summary rather than its historical unread state.

Removal sends `chat.conversationRemoved` with only `eventId` and
`conversationId`. A delayed removal retry does not evict someone who has rejoined.
Future notification invalidations share `/me`; there is no separate notification
socket or persistent notification inbox in this phase.

Deduplicate by source `eventId` separately for conversation and personal
purposes. Apply only newer `summaryVersion` snapshots so an older badge cannot
undo a read response. The worker's bounded dedup cache helps partial retries,
but is not durable across process restart. Joining a room always triggers REST
resynchronization; socket delivery is not a client receipt.

## Prepared-event runtime

Workers can publish to `ChattingHubs.Conversations` and `ChattingHubs.Personal` using
`realtime.ServerEvent`; missing rooms are skipped. Supply `RecipientUserID` for
private conversation events. The personal room itself is already user-scoped.
Do not publish message bodies, media URLs or private read/preferences as a
room-wide event. The worker is responsible for current recipients, DTO creation,
durable ordering/retry and authorization; the room only delivers prepared data.

Run `go run . migrate` before starting this version against an existing schema.
Migration adds claim leases, session-denylist metadata, durable replay floors
and pending/retention indexes. No Redis stream is required for chat dispatch.

Swagger describes the HTTP upgrade/statuses. Test clients should open a real
WebSocket; the docs UI's ordinary HTTP request does not perform an upgrade.
