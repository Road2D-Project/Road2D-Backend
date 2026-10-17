// Wall-clock script. Freeze windows do not advance the rider along the polyline.

import { pointAt, offsetMeters, type Route } from "./path.js";
import {
  batteryOf,
  clampBattery,
  deviateMeters,
  evDeviate,
  evDisconnect,
  evLowBattery,
  evPause,
  evStayMode,
  evWeakSignal,
  signalOf,
  type Motion,
  type ScenarioEvent,
} from "./scenario.js";

export const timeEps = 1e-6;

export interface Effect {
  byName: Map<string, ScenarioEvent>;
  order: string[];
}

export function emptyEffect(): Effect {
  return { byName: new Map(), order: [] };
}

export function effectsAt(events: ScenarioEvent[], elapsed: number): Effect {
  const fx = emptyEffect();
  const seen = new Set<string>();
  for (const event of events) {
    if (elapsed < event.at || elapsed >= event.at + event.duration) {
      continue;
    }
    if (!seen.has(event.name)) {
      seen.add(event.name);
      fx.order.push(event.name);
    }
    fx.byName.set(event.name, event);
  }
  return fx;
}

function freezes(name: string): boolean {
  return name === evDisconnect || name === evPause || name === evStayMode;
}

export function frozenSeconds(events: ScenarioEvent[], until: number): number {
  if (until <= 0) {
    return 0;
  }
  const spans: Array<[number, number]> = [];
  for (const event of events) {
    if (!freezes(event.name)) {
      continue;
    }
    let start = event.at;
    let end = event.at + event.duration;
    if (start < 0) {
      start = 0;
    }
    if (end > until) {
      end = until;
    }
    if (end <= start) {
      continue;
    }
    spans.push([start, end]);
  }
  if (spans.length === 0) {
    return 0;
  }
  spans.sort((a, b) => a[0] - b[0]);
  let curStart = spans[0][0];
  let curEnd = spans[0][1];
  let total = 0;
  for (const span of spans.slice(1)) {
    if (span[0] <= curEnd) {
      if (span[1] > curEnd) {
        curEnd = span[1];
      }
      continue;
    }
    total += curEnd - curStart;
    curStart = span[0];
    curEnd = span[1];
  }
  return total + curEnd - curStart;
}

export function alongMeters(motion: Motion, events: ScenarioEvent[], elapsed: number, length: number): number {
  let moved = elapsed - frozenSeconds(events, elapsed);
  if (moved < 0) {
    moved = 0;
  }
  let dist = moved * motion.speedMps;
  if (length > 0 && motion.loop) {
    return dist % length;
  }
  if (dist > length) {
    return length;
  }
  return dist;
}

export interface Sample {
  point: { lat: number; lng: number };
  bearing: number;
  alongM: number;
  battery: number;
  signal: number;
  effect: Effect;
}

export function sampleAt(route: Route, motion: Motion, events: ScenarioEvent[], elapsed: number): Sample {
  const alongM = alongMeters(motion, events, elapsed, route.length);
  const placed = pointAt(route, alongM);
  const effect = effectsAt(events, elapsed);
  let point = placed.point;
  const deviate = effect.byName.get(evDeviate);
  if (deviate) {
    point = offsetMeters(point, placed.bearing, deviateMeters(deviate));
  }
  let battery = clampBattery(motion.batteryPercent);
  let signal = motion.signalDbm;
  const low = effect.byName.get(evLowBattery);
  if (low) {
    battery = batteryOf(low);
  }
  const weak = effect.byName.get(evWeakSignal);
  if (weak) {
    signal = signalOf(weak);
  }
  return { point, bearing: placed.bearing, alongM, battery, signal, effect };
}

export function horizon(route: Route, motion: Motion, events: ScenarioEvent[]): number {
  let last = 0;
  for (const event of events) {
    const end = event.at + event.duration;
    if (end > last) {
      last = end;
    }
  }
  if (motion.loop) {
    return last === 0 ? 60 : last;
  }
  const moved = (last - frozenSeconds(events, last)) * motion.speedMps;
  if (moved + 0.01 < route.length) {
    last += (route.length - moved) / motion.speedMps;
  }
  return last;
}

export function nextMark(elapsed: number, interval: number, events: ScenarioEvent[]): number {
  let next = Number.POSITIVE_INFINITY;
  if (interval > 0) {
    const step = Math.floor((elapsed + timeEps) / interval) + 1;
    next = step * interval;
  }
  for (const event of events) {
    for (const mark of [event.at, event.at + event.duration]) {
      if (mark > elapsed + timeEps && mark < next) {
        next = mark;
      }
    }
  }
  if (!Number.isFinite(next)) {
    return elapsed + interval;
  }
  return next;
}

export function turnedOn(prev: Effect, cur: Effect): ScenarioEvent[] {
  const out: ScenarioEvent[] = [];
  for (const name of cur.order) {
    if (prev.byName.has(name)) {
      continue;
    }
    const event = cur.byName.get(name);
    if (event) {
      out.push(event);
    }
  }
  return out;
}

export function turnedOff(prev: Effect, cur: Effect): string[] {
  const out: string[] = [];
  for (const name of prev.order) {
    if (!cur.byName.has(name)) {
      out.push(name);
    }
  }
  return out;
}
