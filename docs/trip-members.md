# Trip membership

A trip is its own roster. It does not store a `groupId`, and deleting a standing group does not look at trips.

The invite token is never returned on the trip payload. `POST /trips/{tripId}/invite-link` mints it. The seeder prints that token next to the rest of the trip.

## Create

`POST /trips`

The caller becomes the leader. `memberUserIds` is required and those accounts are seated immediately as members. There is no friend graph yet, so any user id is accepted. Optional `mainBranch` is the response of `POST /planning/preview`. That reviewed location route becomes the trip's initial main branch.

`tripType` freezes `memberLimit`. Today only `bronze` has a policy: 15 seats, counting the leader. `silver`, `gold`, and `diamond` are reserved names; creating or forking them returns 400 until a subscription policy sets their cap. `PATCH /trips/{tripId}` can change name, note, times, and `visibility`. It cannot change `tripType` or `memberLimit`.

`visibility` true means any authenticated user can find the trip (`GET /trips/public` and `GET /trips/{tripId}`). False means only an active member can read it; everyone else gets 404.

## Adding someone later

Both paths need a leader or an admin before the person has a seat.

| Path | Who calls it | First status | Who finishes it |
| --- | --- | --- | --- |
| Invite link `POST /trips/join/{token}` | The person with the link | `pending` | Leader or admin `POST /trips/{tripId}/join-requests/{userId}?action=accept` |
| Member invite `POST /trips/{tripId}/invite/{userId}` | Any active member | `pending` | Same approval |
| Leader or admin invite | Leader or admin | `invited` | The invitee `POST /trips/{tripId}/invitations?action=accept` |

Accept and approve both refuse with 409 when the active roster is already at `memberLimit`. A left, rejected, or kicked row is reused instead of inserting a second seat.

`GET /trips/invitations` is the caller's invited rows. `GET /trips/{tripId}/join-requests` is the pending queue.

## Edit, kick, leave

- `PATCH /trips/{tripId}/members/{userId}` — nickname is only your own. The leader may set another member's role to `admin` or `member`. Leadership is not assigned here.
- `DELETE /trips/{tripId}/members/{userId}` — leader or admin kicks. Nobody kicks the leader. An admin does not kick another admin. The row stays `kicked`.
- `POST /trips/{tripId}/leave` — a leader's seat moves to the earliest-joined admin, otherwise the earliest-joined member. The last person deletes the trip instead.

## Fork

`POST /trips/{tripId}/fork`

Any active member may fork. The caller is the leader of the new trip. Other active members are copied as members, unless their ids are in `excludeUserIds`. The caller cannot be dropped. Omitted name, note, times, type, and visibility are copied from the source. The new `memberLimit` is computed from the new trip's type, not sent by the client. If the copied roster does not fit, exclude more people. The route graph is not copied; the new trip starts with an empty draft branch.

## Not built yet

### Friend system

Create and invite take raw user ids. When friends exist, those calls should reject an account that is not a friend of the caller. Nothing in this module checks that today.

### Group chat

Creating a trip should also create one group chat in the same step. The chat and the trip are parallel objects: they are not linked by `groupId`, and their member lists are the same people. That chat is not implemented.

When it is, keep the two rosters aligned on:

- create (leader plus the required members)
- approve a join request
- accept an invitation
- kick
- leave
- fork (a new chat for the new trip, not the source chat)
