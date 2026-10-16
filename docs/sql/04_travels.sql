-- Travel là một hop của một trip: (trip, from destination, to destination, vehicle).
-- Leg là cache đường dùng chung theo tọa độ + vehicle. Travel.leg_id trỏ vào đó khi leg đã có id Postgres.
-- vehicle lưu lowercase: car | bike | truck | taxi | hd. Giá trị 0 của enum là vehicleunknown.
-- source của leg: direction | trip.

-- Hop đã lưu của trip đang xem, kèm chặng leg nếu có.
SELECT
    tr.id AS travel_id,
    src.name AS from_name,
    dst.name AS to_name,
    tr.vehicle,
    tr.distance_m,
    tr.duration_s,
    tr.is_frozen,
    tr.leg_id,
    tr.last_computed_at,
    length(tr.polyline) AS polyline_chars,
    left(tr.polyline, 48) AS polyline_prefix
FROM road_to_destination.travels AS tr
JOIN road_to_destination.destinations AS src ON src.id = tr.from_destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = tr.to_destination_id
WHERE tr.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
ORDER BY src.name, dst.name, tr.vehicle;

-- travels[i] trên graph: hop từ stop i sang stop i+1. Thiếu travel thì travel_id NULL.
SELECT
    row_number() OVER (ORDER BY b.created_at, b.id) - 1 AS branch_index,
    b.label,
    hop.order_in_branch AS from_order,
    src.name AS from_name,
    dst.name AS to_name,
    tr.id AS travel_id,
    tr.vehicle,
    tr.distance_m,
    tr.duration_s,
    length(tr.polyline) AS polyline_chars
FROM road_to_destination.trip_branches AS b
JOIN road_to_destination.branch_destinations AS hop ON hop.trip_branch_id = b.id
JOIN road_to_destination.branch_destinations AS nxt
    ON nxt.trip_branch_id = b.id
   AND nxt.order_in_branch = hop.order_in_branch + 1
JOIN road_to_destination.destinations AS src ON src.id = hop.destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = nxt.destination_id
LEFT JOIN road_to_destination.travels AS tr
    ON tr.trip_id = b.trip_id
   AND tr.from_destination_id = hop.destination_id
   AND tr.to_destination_id = nxt.destination_id
WHERE b.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
  AND b.is_draft = false
ORDER BY b.created_at, b.id, hop.order_in_branch;

-- Polyline đầy đủ của một hop, để paste vào client test. Lấy hop dài nhất của trip đang xem.
SELECT
    tr.id AS travel_id,
    src.name AS from_name,
    dst.name AS to_name,
    tr.vehicle,
    tr.distance_m,
    tr.polyline
FROM road_to_destination.travels AS tr
JOIN road_to_destination.destinations AS src ON src.id = tr.from_destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = tr.to_destination_id
WHERE tr.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
ORDER BY length(tr.polyline) DESC
LIMIT 1;

-- Leg cache. steps là jsonb các RouteStep. polyline của leg là các step đã ghép.
SELECT
    l.id,
    l.vehicle,
    l.source,
    l.from_lat,
    l.from_lng,
    l.to_lat,
    l.to_lng,
    l.distance_m,
    l.duration_s,
    l.ttl_seconds,
    l.last_computed_at,
    length(l.polyline) AS polyline_chars,
    CASE
        WHEN l.steps IS NULL THEN 0
        ELSE jsonb_array_length(l.steps::jsonb)
    END AS step_count,
    l.steps -> 0 ->> 'instruction' AS first_instruction
FROM road_to_destination.legs AS l
ORDER BY l.last_computed_at DESC;

-- Leg mà travel đang trỏ tới, để so polyline đã chụp với cache.
SELECT
    tr.id AS travel_id,
    src.name AS from_name,
    dst.name AS to_name,
    length(tr.polyline) AS travel_polyline_chars,
    length(l.polyline) AS leg_polyline_chars,
    tr.polyline = l.polyline AS polyline_matches_leg,
    CASE
        WHEN l.steps IS NULL THEN 0
        ELSE jsonb_array_length(l.steps::jsonb)
    END AS leg_step_count,
    l.source
FROM road_to_destination.travels AS tr
JOIN road_to_destination.destinations AS src ON src.id = tr.from_destination_id
JOIN road_to_destination.destinations AS dst ON dst.id = tr.to_destination_id
LEFT JOIN road_to_destination.legs AS l ON l.id = tr.leg_id
WHERE tr.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
ORDER BY src.name, dst.name;
