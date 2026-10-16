-- Các câu này trả về dòng lỗi. Kết quả rỗng là ổn.
-- Chạy cả file, hoặc từng khối SELECT.

-- Nhiều hơn một leader active trên cùng trip.
SELECT
    t.id AS trip_id,
    t.name,
    count(*) AS active_leaders
FROM road_to_destination.trips AS t
JOIN road_to_destination.trip_members AS m ON m.trip_id = t.id
WHERE m.role = 'leader'
  AND m.status = 'active'
GROUP BY t.id, t.name
HAVING count(*) <> 1;

-- Roster active vượt member_limit.
SELECT
    t.id AS trip_id,
    t.name,
    t.member_limit,
    count(*) AS active_members
FROM road_to_destination.trips AS t
JOIN road_to_destination.trip_members AS m ON m.trip_id = t.id
WHERE m.status = 'active'
GROUP BY t.id, t.name, t.member_limit
HAVING count(*) > t.member_limit;

-- Nhánh đường (không phải draft) có ít hơn 2 stop.
SELECT
    t.name AS trip_name,
    b.id AS branch_id,
    b.label,
    count(s.id) AS stop_count
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.trips AS t ON t.id = b.trip_id
LEFT JOIN road_to_destination.branch_destinations AS s ON s.trip_branch_id = b.id
WHERE b.is_draft = false
GROUP BY t.name, b.id, b.label
HAVING count(s.id) < 2;

-- order_in_branch không bắt đầu từ 0, hoặc bị đứt số.
SELECT
    t.name AS trip_name,
    b.id AS branch_id,
    b.label,
    b.is_draft,
    count(s.id) AS stop_count,
    min(s.order_in_branch) AS min_order,
    max(s.order_in_branch) AS max_order
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.trips AS t ON t.id = b.trip_id
JOIN road_to_destination.branch_destinations AS s ON s.trip_branch_id = b.id
GROUP BY t.name, b.id, b.label, b.is_draft
HAVING min(s.order_in_branch) <> 0
    OR max(s.order_in_branch) <> count(s.id) - 1;

-- Stop đầu không khớp split, hoặc stop cuối không khớp merge.
WITH bounds AS (
    SELECT
        t.name AS trip_name,
        b.id AS branch_id,
        b.label,
        b.split_from_destination_id,
        b.merge_to_destination_id,
        (
            SELECT s.destination_id
            FROM road_to_destination.branch_destinations AS s
            WHERE s.trip_branch_id = b.id
            ORDER BY s.order_in_branch
            LIMIT 1
        ) AS first_stop,
        (
            SELECT s.destination_id
            FROM road_to_destination.branch_destinations AS s
            WHERE s.trip_branch_id = b.id
            ORDER BY s.order_in_branch DESC
            LIMIT 1
        ) AS last_stop
    FROM road_to_destination.trip_branches AS b
    JOIN road_to_destination.trips AS t ON t.id = b.trip_id
    WHERE b.is_draft = false
)
SELECT
    trip_name,
    branch_id,
    label,
    split_from_destination_id,
    first_stop,
    merge_to_destination_id,
    last_stop
FROM bounds
WHERE (
    split_from_destination_id IS NOT NULL
    AND split_from_destination_id IS DISTINCT FROM first_stop
)
   OR (
       merge_to_destination_id IS NOT NULL
       AND merge_to_destination_id IS DISTINCT FROM last_stop
   );

-- Hai stop liên tiếp trên nhánh đường nhưng chưa có travel.
SELECT
    t.name AS trip_name,
    b.label,
    hop.order_in_branch,
    src.name AS from_name,
    dst.name AS to_name
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.trips AS t ON t.id = b.trip_id
JOIN road_to_destination.branch_destinations AS hop ON hop.trip_branch_id = b.id
JOIN road_to_destination.branch_destinations AS nxt
    ON nxt.trip_branch_id = b.id
   AND nxt.order_in_branch = hop.order_in_branch + 1
JOIN road_to_destination.destinations AS src ON src.id = hop.destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = nxt.destination_id
WHERE b.is_draft = false
  AND NOT EXISTS (
      SELECT 1
      FROM road_to_destination.travels AS tr
      WHERE tr.trip_id = b.trip_id
        AND tr.from_destination_id = hop.destination_id
        AND tr.to_destination_id = nxt.destination_id
  )
ORDER BY t.name, b.created_at, hop.order_in_branch;

-- Travel không nằm trên hai stop liên tiếp của một nhánh đường cùng trip.
SELECT
    t.name AS trip_name,
    tr.id AS travel_id,
    src.name AS from_name,
    dst.name AS to_name,
    tr.vehicle
FROM road_to_destination.travels AS tr
JOIN road_to_destination.trips AS t ON t.id = tr.trip_id
JOIN road_to_destination.destinations AS src ON src.id = tr.from_destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = tr.to_destination_id
WHERE NOT EXISTS (
    SELECT 1
    FROM road_to_destination.trip_branches AS b
    JOIN road_to_destination.branch_destinations AS hop ON hop.trip_branch_id = b.id
    JOIN road_to_destination.branch_destinations AS nxt
        ON nxt.trip_branch_id = b.id
       AND nxt.order_in_branch = hop.order_in_branch + 1
    WHERE b.trip_id = tr.trip_id
      AND b.is_draft = false
      AND hop.destination_id = tr.from_destination_id
      AND nxt.destination_id = tr.to_destination_id
);

-- Leg có step nhưng polyline rỗng. Lúc lưu, polyline phải là các step ghép lại.
SELECT
    l.id,
    l.vehicle,
    l.source,
    jsonb_array_length(l.steps::jsonb) AS step_count,
    length(COALESCE(l.polyline, '')) AS polyline_chars
FROM road_to_destination.legs AS l
WHERE l.steps IS NOT NULL
  AND jsonb_array_length(l.steps::jsonb) > 0
  AND COALESCE(l.polyline, '') = '';

-- assigned_branch_id trỏ nhánh draft, hoặc nhánh của trip khác.
SELECT
    t.name AS trip_name,
    u.username,
    m.assigned_branch_id,
    branch.trip_id AS branch_trip_id,
    branch.is_draft,
    branch.label
FROM road_to_destination.trip_members AS m
JOIN road_to_destination.trips AS t ON t.id = m.trip_id
JOIN road_to_destination.users AS u ON u.id = m.user_id
LEFT JOIN road_to_destination.trip_branches AS branch ON branch.id = m.assigned_branch_id
WHERE m.assigned_branch_id IS NOT NULL
  AND (
      branch.id IS NULL
      OR branch.trip_id <> m.trip_id
      OR branch.is_draft = true
  );
