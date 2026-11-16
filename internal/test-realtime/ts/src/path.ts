// Geometry and Google/Goong polyline (precision 1e5), matching the Go client.

export interface Point {
  lat: number;
  lng: number;
}

export interface Route {
  points: Point[];
  cumM: number[];
  length: number;
  source: string;
}

const earthRadiusM = 6371000;

export function decode(encoded: string): Point[] {
  encoded = encoded.trim();
  if (!encoded) {
    throw new Error("polyline rỗng");
  }
  const points: Point[] = [];
  let lat = 0;
  let lng = 0;
  let index = 0;
  while (index < encoded.length) {
    const latStep = decodeValue(encoded, index);
    index = latStep.index;
    const lngStep = decodeValue(encoded, index);
    index = lngStep.index;
    lat += latStep.value;
    lng += lngStep.value;
    points.push({ lat: lat / 1e5, lng: lng / 1e5 });
  }
  return points;
}

export function encode(points: Point[]): string {
  let lat = 0;
  let lng = 0;
  let out = "";
  for (const point of points) {
    const nextLat = Math.round(point.lat * 1e5);
    const nextLng = Math.round(point.lng * 1e5);
    out += encodeSigned(nextLat - lat);
    out += encodeSigned(nextLng - lng);
    lat = nextLat;
    lng = nextLng;
  }
  return out;
}

function decodeValue(encoded: string, index: number): { value: number; index: number } {
  let result = 0;
  let shift = 0;
  for (;;) {
    if (index >= encoded.length) {
      throw new Error("polyline không hợp lệ");
    }
    const bite = encoded.charCodeAt(index) - 63;
    index++;
    result += (bite & 0x1f) * 2 ** shift;
    shift += 5;
    if (bite < 0x20) {
      break;
    }
    if (shift > 60) {
      throw new Error("polyline không hợp lệ");
    }
  }
  const value = result % 2 !== 0 ? -Math.floor(result / 2) - 1 : Math.floor(result / 2);
  return { value, index };
}

function encodeSigned(value: number): string {
  let zigzag = value < 0 ? -value * 2 - 1 : value * 2;
  let out = "";
  while (zigzag >= 32) {
    const chunk = zigzag % 32;
    out += String.fromCharCode((0x20 | chunk) + 63);
    zigzag = Math.floor(zigzag / 32);
  }
  out += String.fromCharCode(zigzag + 63);
  return out;
}

export function buildRoute(points: Point[], source: string): Route {
  if (points.length === 0) {
    throw new Error("đường không có điểm");
  }
  const cleaned: Point[] = [];
  for (const point of points) {
    if (!validCoord(point)) {
      throw new Error(`tọa độ ngoài phạm vi: lat ${point.lat.toFixed(6)} lng ${point.lng.toFixed(6)}`);
    }
    if (cleaned.length > 0 && metersBetween(cleaned[cleaned.length - 1], point) < 0.5) {
      continue;
    }
    cleaned.push(point);
  }
  if (cleaned.length < 2) {
    throw new Error("đường cần ít nhất 2 điểm khác nhau");
  }
  const cumM = new Array<number>(cleaned.length).fill(0);
  for (let i = 1; i < cleaned.length; i++) {
    cumM[i] = cumM[i - 1] + metersBetween(cleaned[i - 1], cleaned[i]);
  }
  if (cumM[cumM.length - 1] <= 0) {
    throw new Error("polyline không có chiều dài");
  }
  return { points: cleaned, cumM, length: cumM[cumM.length - 1], source };
}

function validCoord(point: Point): boolean {
  return point.lat >= -90 && point.lat <= 90 && point.lng >= -180 && point.lng <= 180;
}

export function metersBetween(a: Point, b: Point): number {
  const lat1 = (a.lat * Math.PI) / 180;
  const lat2 = (b.lat * Math.PI) / 180;
  const dLat = lat2 - lat1;
  const dLng = ((b.lng - a.lng) * Math.PI) / 180;
  let h =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) * Math.sin(dLng / 2);
  if (h > 1) {
    h = 1;
  }
  return 2 * earthRadiusM * Math.asin(Math.sqrt(h));
}

// bearing is radians clockwise from north.
export function bearing(a: Point, b: Point): number {
  const lat1 = (a.lat * Math.PI) / 180;
  const lat2 = (b.lat * Math.PI) / 180;
  const dLng = ((b.lng - a.lng) * Math.PI) / 180;
  const y = Math.sin(dLng) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLng);
  return Math.atan2(y, x);
}

export function pointAt(route: Route, dist: number): { point: Point; bearing: number } {
  const n = route.points.length;
  if (dist <= 0) {
    return { point: route.points[0], bearing: bearing(route.points[0], route.points[1]) };
  }
  if (dist >= route.length) {
    return {
      point: route.points[n - 1],
      bearing: bearing(route.points[n - 2], route.points[n - 1]),
    };
  }
  let i = 1;
  while (i < n - 1 && route.cumM[i] < dist) {
    i++;
  }
  const span = route.cumM[i] - route.cumM[i - 1];
  const t = span > 0 ? (dist - route.cumM[i - 1]) / span : 0;
  return {
    point: {
      lat: route.points[i - 1].lat + (route.points[i].lat - route.points[i - 1].lat) * t,
      lng: route.points[i - 1].lng + (route.points[i].lng - route.points[i - 1].lng) * t,
    },
    bearing: bearing(route.points[i - 1], route.points[i]),
  };
}

// Negative meters shift the point to the left of travel.
export function offsetMeters(point: Point, bearingRad: number, meters: number): Point {
  const turn = bearingRad + Math.PI / 2;
  const north = meters * Math.cos(turn);
  const east = meters * Math.sin(turn);
  return {
    lat: point.lat + north / 111320,
    lng: point.lng + east / (111320 * Math.cos((point.lat * Math.PI) / 180)),
  };
}
