// Checks the polyline codec and the sample file without opening a socket.

import { fileURLToPath } from "node:url";

import { decode, encode } from "./path.js";
import { loadScenario, normalize, scenarioRoute } from "./scenario.js";

const known = "_p~iF~ps|U_ulLnnqC_mqNvxq`@";
const knownPoints = [
  { lat: 38.5, lng: -120.2 },
  { lat: 40.7, lng: -120.95 },
  { lat: 43.252, lng: -126.453 },
];

if (encode(knownPoints) !== known) {
  throw new Error(`encode ${encode(knownPoints)}`);
}
const decoded = decode(known);
if (decoded.length !== knownPoints.length) {
  throw new Error(`decoded ${decoded.length}`);
}
for (let i = 0; i < knownPoints.length; i++) {
  if (Math.abs(decoded[i].lat - knownPoints[i].lat) > 1e-5 || Math.abs(decoded[i].lng - knownPoints[i].lng) > 1e-5) {
    throw new Error(`point ${i} ${JSON.stringify(decoded[i])}`);
  }
}

const home = [
  { lat: 10.7725, lng: 106.698 },
  { lat: 10.7742, lng: 106.7008 },
  { lat: 10.7761, lng: 106.7034 },
  { lat: 10.7784, lng: 106.7051 },
  { lat: 10.7809, lng: 106.7076 },
  { lat: 10.7832, lng: 106.7092 },
];
const sc = loadScenario(fileURLToPath(new URL("../../scenario.home.json", import.meta.url)));
normalize(sc);
if (sc.path.polyline !== encode(home)) {
  throw new Error(`home polyline ${sc.path.polyline}`);
}
const route = scenarioRoute(sc);
if (route.source !== "polyline" || route.points.length !== home.length || sc.events.length !== 7) {
  throw new Error(`route ${route.source} ${route.points.length}`);
}
if (route.length < 1500 || route.length > 2000) {
  throw new Error(`length ${route.length}`);
}

console.log(`ok  polyline ${route.points.length} điểm, ${Math.round(route.length)} m, ${sc.events.length} event`);
