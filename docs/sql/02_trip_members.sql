-- Thành viên của trip cập nhật gần nhất.
-- status: invited | pending | active | rejected | left | kicked
-- role:   leader | member | admin

SELECT
    t.name AS trip_name,
    u.username,
    u.email,
    m.nickname,
    m.role,
    m.status,
    m.invitor_name,
    m.joined_at,
    m.assigned_branch_id,
    branch.label AS assigned_branch,
    branch.is_draft AS assigned_branch_is_draft
FROM road_to_destination.trip_members AS m
JOIN road_to_destination.trips AS t ON t.id = m.trip_id
JOIN road_to_destination.users AS u ON u.id = m.user_id
LEFT JOIN road_to_destination.trip_branches AS branch ON branch.id = m.assigned_branch_id
WHERE m.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
ORDER BY
    CASE m.role
        WHEN 'leader' THEN 0
        WHEN 'admin' THEN 1
        ELSE 2
    END,
    m.joined_at NULLS LAST,
    u.username;

-- Hàng đợi join: status = pending. Leader/admin duyệt các dòng này.
SELECT
    t.name AS trip_name,
    u.username,
    m.nickname,
    m.invitor_name,
    m.updated_at
FROM road_to_destination.trip_members AS m
JOIN road_to_destination.trips AS t ON t.id = m.trip_id
JOIN road_to_destination.users AS u ON u.id = m.user_id
WHERE m.status = 'pending'
ORDER BY m.updated_at DESC;

-- Lời mời đang chờ người được mời chấp nhận: status = invited.
SELECT
    t.name AS trip_name,
    u.username,
    m.nickname,
    m.invitor_name,
    m.updated_at
FROM road_to_destination.trip_members AS m
JOIN road_to_destination.trips AS t ON t.id = m.trip_id
JOIN road_to_destination.users AS u ON u.id = m.user_id
WHERE m.status = 'invited'
ORDER BY m.updated_at DESC;

-- Ghế active so với member_limit. active_over_limit > 0 là roster vượt trần.
SELECT
    t.name,
    t.trip_type,
    t.member_limit,
    count(*) FILTER (WHERE m.status = 'active') AS active_members,
    count(*) FILTER (WHERE m.status = 'active') - t.member_limit AS active_over_limit
FROM road_to_destination.trips AS t
LEFT JOIN road_to_destination.trip_members AS m ON m.trip_id = t.id
GROUP BY t.id, t.name, t.trip_type, t.member_limit
ORDER BY active_over_limit DESC, t.name;
