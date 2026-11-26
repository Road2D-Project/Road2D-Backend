# Chatting sockets

Phase 1 step 8 adds two WebSocket endpoints. `module/conversation` owns REST,
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

Message/read/preferences mutations still use the existing conversation REST
APIs. The outbox dispatcher and committed membership/logout revocation wiring
belong to step 9. At this stage REST message writes do **not** automatically
produce socket events. Prepared publishers can already deliver frames through
the two hubs, and typing/resync work live.

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

`JoinWithOptions` carries the verified access token's `jti`, session version
and expiry into the connection. An expiry timer and read/write deadlines close
it at token expiry, even if pongs keep arriving. Reconnect using a refreshed
access token; there is no token-refresh command on the socket. The `jti` is an
access-token identifier, not a shared login ID with its refresh token.

Logout/session-version changes and membership removal are checked on a new
handshake. Closing already-open sockets immediately for these changes, and the
join-versus-revocation race guard, are still step 9 work. Token expiry alone is
not that revocation mechanism.

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

## Prepared durable-event delivery

Workers can publish to `ChattingHubs.Conversations` and `ChattingHubs.Personal` using
`realtime.ServerEvent`; missing rooms are skipped. Supply `RecipientUserID` for
private conversation events. The personal room itself is already user-scoped.
Do not publish message bodies, media URLs or private read/preferences as a
room-wide event. The worker is responsible for current recipients, DTO creation,
durable ordering/retry and authorization; the room only delivers prepared data.

The intended personal types are `chat.conversationUpdated` and
`chat.conversationRemoved`. Future notification invalidations share `/me`;
there is no separate notification socket or persistent inbox in this step.
Deduplicate separately for conversation and personal purposes, and apply only
newer `summaryVersion` values so an older badge cannot undo a read response.

Swagger describes the HTTP upgrade/statuses. Test clients should open a real
WebSocket; the docs UI's ordinary HTTP request does not perform an upgrade.
