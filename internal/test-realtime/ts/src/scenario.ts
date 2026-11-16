// The mock file: one rider, one path, and timed events.

import { readFileSync } from "node:fs";

import { buildRoute, decode, type Point, type Route } from "./path.js";

export const evDisconnect = "disconnect";
export const evPause = "pause";
export const evDeviate = "deviate";
export const evWeakSignal = "weakSignal";
export const evLowBattery = "lowBattery";
export const evStayMode = "stayMode";
export const evUserStayPublic = "userStayPublic";

const eventOrder = [
  evDisconnect,
  evPause,
  evDeviate,
  evWeakSignal,
  evLowBattery,
  evStayMode,
  evUserStayPublic,
];

const eventAlias: Record<string, string> = {
  disconnect: evDisconnect,
  pause: evPause,
  deviate: evDeviate,
  weaksignal: evWeakSignal,
  lowbattery: evLowBattery,
  staymode: evStayMode,
  userstaypublic: evUserStayPublic,
};

export interface Auth {
  accessToken: string;
  username: string;
  password: string;
}

export interface Motion {
  speedMps: number;
  sendEveryMs: number;
  batteryPercent: number;
  signalDbm: number;
  loop: boolean;
}

export interface ScenarioEvent {
  name: string;
  at: number;
  duration: number;
  offsetMeters: number;
  signalDbm: number | null;
  batteryPercent: number | null;
  note: string;
}

export interface Scenario {
  name: string;
  description: string;
  server: string;
  roomId: string;
  auth: Auth;
  motion: Motion;
  path: { polyline: string; points: Point[] };
  events: ScenarioEvent[];
}

export function loadScenario(path: string): Scenario {
  let body = readFileSync(path);
  if (body.length >= 3 && body[0] === 0xef && body[1] === 0xbb && body[2] === 0xbf) {
    body = body.subarray(3);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(body.toString("utf8"));
  } catch (err) {
    throw new Error(`JSON ${path}: ${err instanceof Error ? err.message : err}`);
  }
  return parseScenario(parsed);
}

export function parseScenario(data: unknown): Scenario {
  if (!isRecord(data)) {
    throw new Error("JSON phải là object");
  }
  const motionRaw = isRecord(data.motion) ? data.motion : {};
  const authRaw = isRecord(data.auth) ? data.auth : {};
  const pathRaw = isRecord(data.path) ? data.path : {};
  return {
    name: stringField(data.name),
    description: stringField(data.description),
    server: stringField(data.server),
    roomId: stringField(data.roomId),
    auth: {
      accessToken: stringField(authRaw.accessToken),
      username: stringField(authRaw.username),
      password: stringField(authRaw.password),
    },
    motion: {
      speedMps: numberField(motionRaw.speedMps, 0),
      sendEveryMs: numberField(motionRaw.sendEveryMs, 0),
      batteryPercent: numberField(motionRaw.batteryPercent, 0),
      signalDbm: numberField(motionRaw.signalDbm, 0),
      loop: motionRaw.loop === true,
    },
    path: {
      polyline: stringField(pathRaw.polyline),
      points: parsePoints(pathRaw.points),
    },
    events: parseEvents(data),
  };
}

function parseEvents(data: Record<string, unknown>): ScenarioEvent[] {
  const events = Array.isArray(data.events) ? data.events : null;
  const event = Array.isArray(data.event) ? data.event : null;
  if (events && events.length > 0 && event && event.length > 0) {
    throw new Error("chỉ điền một trong hai khóa events hoặc event");
  }
  const rows = events && events.length > 0 ? events : event && event.length > 0 ? event : events ?? event ?? [];
  return rows.map((row, index) => {
    try {
      return parseEvent(row);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      throw new Error(`events[${index}]: ${message}`);
    }
  });
}

function parseEvent(data: unknown): ScenarioEvent {
  if (!isRecord(data)) {
    throw new Error("event phải là object");
  }
  const name = canonicalEvent(stringField(data.name));
  const at = oneFloat("at", data.at, data.atSec);
  const duration = oneFloat("duration", data.duration, data.durationSec);
  if (at == null) {
    throw new Error("thiếu at (giây kích hoạt, tính từ lúc chạy)");
  }
  if (duration == null) {
    throw new Error("thiếu duration (số giây hiệu lực)");
  }
  if (at < 0) {
    throw new Error(`at phải >= 0, nhận ${at}`);
  }
  if (duration <= 0) {
    throw new Error(`duration phải > 0, nhận ${duration}`);
  }
  return {
    name,
    at,
    duration,
    offsetMeters: numberField(data.offsetMeters, 0),
    signalDbm: optionalInt(data.signalDbm, "signalDbm"),
    batteryPercent: optionalInt(data.batteryPercent, "batteryPercent"),
    note: stringField(data.note).trim(),
  };
}

function oneFloat(label: string, primary: unknown, alias: unknown): number | null {
  const first = optionalNumber(primary);
  const second = optionalNumber(alias);
  if (first != null && second != null && first !== second) {
    throw new Error(`${label} và ${label}Sec khác nhau`);
  }
  return first ?? second;
}

function canonicalEvent(name: string): string {
  const key = name.trim().toLowerCase();
  if (!key) {
    throw new Error("thiếu name");
  }
  const canonical = eventAlias[key];
  if (!canonical) {
    throw new Error(`name ${JSON.stringify(name)} không hỗ trợ (${eventOrder.join(", ")})`);
  }
  return canonical;
}

// Zero means omitted for speed, interval, battery, and signal.
export function normalize(sc: Scenario): void {
  sc.server = sc.server.trim().replace(/\/+$/, "");
  if (!sc.server) {
    sc.server = "http://127.0.0.1:8080";
  }
  sc.roomId = sc.roomId.trim();
  sc.auth.accessToken = sc.auth.accessToken.trim();
  sc.auth.username = sc.auth.username.trim();
  sc.name = sc.name.trim();
  sc.description = sc.description.trim();
  if (sc.motion.speedMps === 0) {
    sc.motion.speedMps = 8;
  }
  if (sc.motion.speedMps <= 0 || sc.motion.speedMps > 80) {
    throw new Error(`speedMps phải trong (0, 80], nhận ${sc.motion.speedMps}`);
  }
  if (sc.motion.sendEveryMs === 0) {
    sc.motion.sendEveryMs = 1000;
  }
  if (sc.motion.sendEveryMs < 200) {
    throw new Error(`sendEveryMs tối thiểu 200, nhận ${sc.motion.sendEveryMs}`);
  }
  if (sc.motion.batteryPercent === 0) {
    sc.motion.batteryPercent = 80;
  }
  if (sc.motion.batteryPercent < 0 || sc.motion.batteryPercent > 100) {
    throw new Error("batteryPercent phải trong 1..100 (0 được hiểu là bỏ trống và thành 80)");
  }
  if (sc.motion.signalDbm === 0) {
    sc.motion.signalDbm = -74;
  }
}

export function scenarioRoute(sc: Scenario): Route {
  if (sc.path.points.length >= 2) {
    return buildRoute(sc.path.points, "points");
  }
  if (sc.path.polyline.trim()) {
    return buildRoute(decode(sc.path.polyline), "polyline");
  }
  throw new Error("path cần polyline hoặc ít nhất 2 điểm trong points");
}

export function describe(event: ScenarioEvent): string {
  switch (event.name) {
    case evDisconnect:
      return "đóng socket, đứng yên trên polyline";
    case evPause:
      return "đứng yên, socket vẫn mở";
    case evDeviate: {
      const meters = deviateMeters(event);
      if (meters < 0) {
        return `lệch trái ${Math.round(-meters)} m`;
      }
      return `lệch phải ${Math.round(meters)} m`;
    }
    case evWeakSignal:
      return `signalDbm=${signalOf(event)}`;
    case evLowBattery:
      return `battery=${batteryOf(event)}%`;
    case evStayMode:
      return "gửi tracking.stayMode true, hết giờ gửi false";
    case evUserStayPublic:
      return "gửi tracking.userStayPublic true, hết giờ gửi false";
    default:
      return event.name;
  }
}

export function deviateMeters(event: ScenarioEvent): number {
  return event.offsetMeters === 0 ? 80 : event.offsetMeters;
}

export function signalOf(event: ScenarioEvent): number {
  return event.signalDbm ?? -120;
}

export function batteryOf(event: ScenarioEvent): number {
  return clampBattery(event.batteryPercent ?? 5);
}

export function clampBattery(percent: number): number {
  if (percent < 0) {
    return 0;
  }
  if (percent > 100) {
    return 100;
  }
  return percent;
}

export function authLabel(sc: Scenario): string {
  if (sc.auth.accessToken) {
    return "access token";
  }
  if (sc.auth.username) {
    return `đăng nhập ${sc.auth.username}`;
  }
  return "chưa cấu hình";
}

function parsePoints(value: unknown): Point[] {
  if (value == null) {
    return [];
  }
  if (!Array.isArray(value)) {
    throw new Error("path.points phải là mảng");
  }
  return value.map((row) => {
    if (!isRecord(row)) {
      throw new Error("điểm phải là object {lat, lng}");
    }
    return { lat: numberField(row.lat, Number.NaN), lng: numberField(row.lng, Number.NaN) };
  });
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function numberField(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function optionalNumber(value: unknown): number | null {
  if (value == null) {
    return null;
  }
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error("at và duration phải là số");
  }
  return value;
}

function optionalInt(value: unknown, label: string): number | null {
  if (value == null) {
    return null;
  }
  if (typeof value !== "number" || !Number.isFinite(value) || !Number.isInteger(value)) {
    throw new Error(`${label} phải là số nguyên`);
  }
  return value;
}
