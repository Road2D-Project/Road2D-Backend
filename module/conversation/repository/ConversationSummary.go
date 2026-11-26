package repository

// Do not initialize read state during GET. Without a saved cursor, derive the
// initial cursor from messages preceding joined_at. Legacy unknown join times
// conservatively start at zero. All fields/counts share one statement snapshot.
const conversationSummarySQL = `WITH summaries AS (
	SELECT c.id, c.trip_id, t.name, tm.role AS my_role,
		(SELECT COUNT(*) FROM trip_members roster WHERE roster.trip_id = c.trip_id AND roster.status = 'active') AS active_member_count,
		read_state.seq AS last_read_seq, s.muted_until,
		(SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id
			AND m.seq > read_state.seq AND m.deleted_at IS NULL
			AND m.kind IN ('text','media','poll') AND m.sender_id IS DISTINCT FROM @userId) AS unread_count,
		c.next_message_seq - 1 AS last_message_seq, c.next_event_seq - 1 AS summary_version,
		latest.id AS last_message_id, latest.revision AS last_message_revision,
		COALESCE(latest.kind, 'system') AS last_message_kind,
		LEFT(latest.text, 160) AS last_message_text,
		(latest.deleted_at IS NOT NULL) AS last_message_deleted,
		COALESCE(latest.created_at, c.created_at) AS last_activity_at
	FROM conversations c
	JOIN trips t ON t.id = c.trip_id
	JOIN trip_members tm ON tm.trip_id = c.trip_id AND tm.user_id = @userId AND tm.status = 'active'
	LEFT JOIN conversation_user_states s ON s.conversation_id = c.id AND s.user_id = @userId
	CROSS JOIN LATERAL (SELECT COALESCE(s.last_read_seq,
		(SELECT MAX(prior.seq) FROM messages prior WHERE prior.conversation_id = c.id AND prior.created_at < tm.joined_at), 0) AS seq) read_state
	LEFT JOIN LATERAL (SELECT m.id, m.kind, m.text, m.revision, m.deleted_at, m.created_at
		FROM messages m WHERE m.conversation_id = c.id ORDER BY m.seq DESC LIMIT 1) latest ON TRUE
)`
