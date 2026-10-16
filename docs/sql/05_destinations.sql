-- Destination là pin. Không có trip_id.
-- Pin thuộc trip nào là do branch_destinations.
-- location_id chỉ có sau khi verify hoặc fork từ Location. lat/lng khi đó là bản sao của location.

-- Pin của trip đang xem, gồm cả inbox draft.
SELECT
    d.id,
    d.name,
    d.status,          -- Editing | Confirm | Deleted
    d.lat,
    d.lng,
    d.stay_over_time,
    d.arrive_time,
    d.location_id,
    loc.name AS location_name,
    loc.place_id,
    loc.is_verified,
    b.label AS branch_label,
    b.is_draft,
    s.order_in_branch
FROM road_to_destination.destinations AS d
JOIN road_to_destination.branch_destinations AS s ON s.destination_id = d.id
JOIN road_to_destination.trip_branches AS b ON b.id = s.trip_branch_id
LEFT JOIN road_to_destination.locations AS loc ON loc.id = d.location_id
WHERE b.trip_id = (
    SELECT id
    FROM road_to_destination.trips
    ORDER BY updated_at DESC
    LIMIT 1
)
ORDER BY b.is_draft, b.created_at, s.order_in_branch;

-- Pin không nằm trên branch nào. Graph và draft đều không thấy các dòng này.
SELECT
    d.id,
    d.name,
    d.status,
    d.lat,
    d.lng,
    d.location_id,
    d.created_at
FROM road_to_destination.destinations AS d
WHERE NOT EXISTS (
    SELECT 1
    FROM road_to_destination.branch_destinations AS s
    WHERE s.destination_id = d.id
)
ORDER BY d.created_at DESC;

-- Location catalog mà pin đang trỏ tới.
SELECT
    loc.id,
    loc.name,
    loc.lat,
    loc.lng,
    loc.place_id,
    loc.formatted_address,
    loc.is_verified,
    loc.compound,
    pt.type_name AS place_type,
    count(d.id) AS destination_count
FROM road_to_destination.locations AS loc
LEFT JOIN road_to_destination.place_types AS pt ON pt.id = loc.place_type_id
LEFT JOIN road_to_destination.destinations AS d ON d.location_id = loc.id
GROUP BY loc.id, pt.type_name
ORDER BY destination_count DESC, loc.name;

-- Pin đã gắn location nhưng tọa độ lệch khỏi location. Sau fork, hai bên phải trùng.
SELECT
    d.id AS destination_id,
    d.name AS destination_name,
    d.lat AS destination_lat,
    d.lng AS destination_lng,
    loc.id AS location_id,
    loc.name AS location_name,
    loc.lat AS location_lat,
    loc.lng AS location_lng
FROM road_to_destination.destinations AS d
JOIN road_to_destination.locations AS loc ON loc.id = d.location_id
WHERE d.lat IS DISTINCT FROM loc.lat
   OR d.lng IS DISTINCT FROM loc.lng
ORDER BY d.name;
