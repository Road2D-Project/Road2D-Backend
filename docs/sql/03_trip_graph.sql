-- Graph của trip cập nhật gần nhất.
-- Khớp GET /v1/trips/:id/graph: bỏ nhánh draft, stops theo order_in_branch.
-- Nhánh tạo trước là nhánh chính (branches[0] lúc PUT graph).

-- Stop trên từng nhánh đường.
SELECT
    row_number() OVER (ORDER BY b.created_at, b.id) - 1 AS branch_index,
    b.id AS branch_id,
    b.label,
    b.is_draft,
    b.split_from_destination_id,
    b.merge_to_destination_id,
    s.order_in_branch,
    d.id AS destination_id,
    d.name,
    d.lat,
    d.lng,
    d.stay_over_time,
    d.status,          -- Editing | Confirm | Deleted
    d.location_id
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.branch_destinations AS s ON s.trip_branch_id = b.id
JOIN road_to_destination.destinations AS d ON d.id = s.destination_id
WHERE b.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
  AND b.is_draft = false
ORDER BY b.created_at, b.id, s.order_in_branch;

-- Cùng graph, dạng JSON gần với response của API. Polyline không nhét vào đây.
SELECT jsonb_pretty(jsonb_build_object(
    'id', t.id,
    'name', t.name,
    'status', t.status,
    'branches', COALESCE((
        SELECT jsonb_agg(
            jsonb_build_object(
                'id', b.id,
                'label', b.label,
                'splitFromDestinationId', b.split_from_destination_id,
                'mergeToDestinationId', b.merge_to_destination_id,
                'stops', COALESCE((
                    SELECT jsonb_agg(
                        jsonb_build_object(
                            'destinationId', d.id,
                            'name', d.name,
                            'lat', d.lat,
                            'lng', d.lng,
                            'stayOverTime', d.stay_over_time,
                            'orderInBranch', s.order_in_branch
                        )
                        ORDER BY s.order_in_branch
                    )
                    FROM road_to_destination.branch_destinations AS s
                    JOIN road_to_destination.destinations AS d ON d.id = s.destination_id
                    WHERE s.trip_branch_id = b.id
                ), '[]'::jsonb)
            )
            ORDER BY b.created_at, b.id
        )
        FROM road_to_destination.trip_branches AS b
        WHERE b.trip_id = t.id
          AND b.is_draft = false
    ), '[]'::jsonb)
)) AS trip_graph
FROM road_to_destination.trips AS t
WHERE t.id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
);

-- Inbox draft. GET /v1/trips/:id/draft đọc nhánh này, graph thì bỏ qua.
SELECT
    b.id AS branch_id,
    b.label,
    s.order_in_branch,
    d.id AS destination_id,
    d.name,
    d.lat,
    d.lng,
    d.status,
    d.location_id
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.branch_destinations AS s ON s.trip_branch_id = b.id
JOIN road_to_destination.destinations AS d ON d.id = s.destination_id
WHERE b.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
  AND b.is_draft = true
ORDER BY s.order_in_branch;

-- Điểm tách / nhập: một destination xuất hiện trên nhiều nhánh đường.
SELECT
    d.id AS destination_id,
    d.name,
    count(DISTINCT b.id) AS route_branch_count,
    array_agg(DISTINCT COALESCE(NULLIF(b.label, ''), b.id::text) ORDER BY COALESCE(NULLIF(b.label, ''), b.id::text)) AS branches
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.branch_destinations AS s ON s.trip_branch_id = b.id
JOIN road_to_destination.destinations AS d ON d.id = s.destination_id
WHERE b.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
  AND b.is_draft = false
GROUP BY d.id, d.name
HAVING count(DISTINCT b.id) > 1
ORDER BY route_branch_count DESC, d.name;
