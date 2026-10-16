-- Trip module. Chạy từng khối trên Supabase SQL Editor.
-- Bảng nằm trong schema road_to_destination.
-- Muốn khóa một trip: bỏ comment dòng WHERE id = '...' trong khối "trip đang xem".

-- Danh sách trip, mới cập nhật trước.
SELECT
    t.id,
    t.name,
    t.status,          -- planning | locked
    t.trip_type,       -- bronze | silver | gold | diamond
    t.member_limit,
    t.visibility,
    t.total_distance,
    t.start_time,
    t.end_time,
    owner.username AS owner_username,
    t.created_at,
    t.updated_at
FROM road_to_destination.trips AS t
JOIN road_to_destination.users AS owner ON owner.id = t.owner_id
ORDER BY t.updated_at DESC;

-- Trip đang xem: bản ghi cập nhật gần nhất.
-- Khóa một id: thay cả khối SELECT id ... LIMIT 1 bằng 'uuid-của-trip'.
SELECT
    t.id,
    t.name,
    t.status,
    t.trip_type,
    t.member_limit,
    t.visibility,
    t.total_distance,
    t.note,
    t.start_time,
    t.end_time,
    t.invite_token,
    t.idempotency_key,
    owner.id AS owner_id,
    owner.username AS owner_username,
    (
        SELECT count(*)
        FROM road_to_destination.trip_members AS m
        WHERE m.trip_id = t.id
          AND m.status = 'active'
    ) AS active_members,
    (
        SELECT count(*)
        FROM road_to_destination.trip_branches AS b
        WHERE b.trip_id = t.id
          AND b.is_draft = false
    ) AS route_branches,
    (
        SELECT count(*)
        FROM road_to_destination.trip_branches AS b
        WHERE b.trip_id = t.id
          AND b.is_draft = true
    ) AS draft_branches
FROM road_to_destination.trips AS t
JOIN road_to_destination.users AS owner ON owner.id = t.owner_id
WHERE t.id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
);

-- Trip public, cùng điều kiện GET /v1/trips/public: visibility = true.
SELECT
    t.id,
    t.name,
    t.status,
    t.trip_type,
    t.member_limit,
    owner.username AS owner_username
FROM road_to_destination.trips AS t
JOIN road_to_destination.users AS owner ON owner.id = t.owner_id
WHERE t.visibility = true
ORDER BY t.updated_at DESC;
