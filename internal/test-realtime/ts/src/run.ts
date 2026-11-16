// Walks the script. Dry-run stops when the path and the events are done.
// Live mode keeps the socket open until Ctrl+C, holding the last point.

import {
  authLabel,
  describe,
  evDisconnect,
  evStayMode,
  evUserStayPublic,
  type Scenario,
} from "./scenario.js";
import {
  boolFrame,
  compactJSON,
  dialSocket,
  Link,
  login,
  messageType,
  positionFrame,
  requireRoom,
  roomURL,
} from "./session.js";
import { emptyEffect, horizon, nextMark, sampleAt, timeEps, turnedOff, turnedOn, type Effect } from "./sim.js";
import type { Route } from "./path.js";

export interface Options {
  dry: boolean;
  quiet: boolean;
}

export type Writer = (line: string) => void;

class Printer {
  constructor(
    private readonly write: Writer,
    readonly quiet: boolean,
  ) {}

  raw(line: string): void {
    this.write(line);
  }

  timed(elapsed: number, kind: string, msg: string): void {
    this.raw(`t=${elapsed.toFixed(1).padStart(6)}s  ${kind.padEnd(5)}  ${msg}`);
  }
}

class Runner {
  private endpoint = "";
  private token = "";
  private link: Link | null = null;
  private socketUp = false;
  private everConnected = false;
  private openedOnce = false;
  private openedThisTick = false;
  private stayOn = false;
  private publicOn = false;
  private lastSend = -1;
  private prev: Effect = emptyEffect();
  private start = 0;
  private readonly end: number;

  constructor(
    private readonly sc: Scenario,
    private readonly route: Route,
    private readonly out: Printer,
    private readonly dry: boolean,
    private readonly signal: AbortSignal,
  ) {
    this.end = horizon(route, sc.motion, sc.events);
  }

  async run(): Promise<void> {
    if (this.dry) {
      try {
        requireRoom(this.sc.roomId);
        this.endpoint = roomURL(this.sc.server, this.sc.roomId);
      } catch {
        this.endpoint = "(chưa có roomId)";
      }
    } else {
      requireRoom(this.sc.roomId);
      this.endpoint = roomURL(this.sc.server, this.sc.roomId);
      this.token = await this.ensureToken();
    }
    this.start = Date.now();
    await this.loop();
  }

  private async ensureToken(): Promise<string> {
    if (this.sc.auth.accessToken) {
      this.out.raw("auth      dùng access token sẵn có");
      return this.sc.auth.accessToken;
    }
    if (!this.sc.auth.username || !this.sc.auth.password) {
      throw new Error("cần accessToken hoặc cả username và password");
    }
    this.out.raw(`auth      đang đăng nhập ${this.sc.auth.username}`);
    return login(this.sc.server, this.sc.auth.username, this.sc.auth.password, this.signal);
  }

  private async loop(): Promise<void> {
    const interval = this.sc.motion.sendEveryMs / 1000;
    let virtual = 0;
    for (;;) {
      if (this.signal.aborted) {
        await this.shutdown(this.elapsed(virtual), "dừng theo Ctrl+C");
        return;
      }
      const elapsed = this.elapsed(virtual);
      try {
        await this.tick(elapsed);
      } catch (err) {
        if (this.signal.aborted) {
          await this.shutdown((Date.now() - this.start) / 1000, "dừng theo Ctrl+C");
          return;
        }
        this.closeLink();
        throw err;
      }
      if (this.dry && elapsed >= this.end - timeEps) {
        await this.shutdown(elapsed, "hết dry-run");
        return;
      }
      let next = nextMark(elapsed, interval, this.sc.events);
      if (this.dry) {
        if (next > this.end) {
          next = this.end;
        }
        if (next <= elapsed + timeEps) {
          await this.shutdown(elapsed, "hết dry-run");
          return;
        }
        virtual = next;
        continue;
      }
      const wait = next - (Date.now() - this.start) / 1000;
      try {
        await sleep(wait, this.signal);
      } catch {
        await this.shutdown((Date.now() - this.start) / 1000, "dừng theo Ctrl+C");
        return;
      }
    }
  }

  private elapsed(virtual: number): number {
    if (this.dry) {
      return virtual;
    }
    return (Date.now() - this.start) / 1000;
  }

  private async tick(elapsed: number): Promise<void> {
    this.openedThisTick = false;
    const current = sampleAt(this.route, this.sc.motion, this.sc.events, elapsed);
    for (const event of turnedOn(this.prev, current.effect)) {
      this.out.timed(
        elapsed,
        "event",
        `${event.name} bật (${event.duration.toFixed(1)}s) — ${describe(event)} · along ${Math.round(current.alongM)} m`,
      );
    }
    for (const name of turnedOff(this.prev, current.effect)) {
      this.out.timed(elapsed, "event", `${name} tắt · along ${Math.round(current.alongM)} m`);
    }

    if (this.socketUp && !this.dry && (this.link == null || this.link.dead)) {
      this.socketUp = false;
      this.link = null;
    }
    const wantOpen = !current.effect.byName.has(evDisconnect);
    if (!wantOpen && this.socketUp) {
      this.closeLink();
    }
    if (wantOpen && !this.socketUp) {
      try {
        await this.open(elapsed);
      } catch (err) {
        if (!this.everConnected) {
          throw err;
        }
        this.out.timed(elapsed, "retry", err instanceof Error ? err.message : String(err));
      }
    }

    await this.syncBool(elapsed, current.effect.byName.has(evStayMode), "stay", "tracking.stayMode");
    await this.syncBool(elapsed, current.effect.byName.has(evUserStayPublic), "public", "tracking.userStayPublic");

    const interval = this.sc.motion.sendEveryMs / 1000;
    const due = this.lastSend < 0 || elapsed - this.lastSend >= interval - timeEps;
    const atEnd = this.dry && elapsed >= this.end - timeEps;
    if (wantOpen && this.socketUp && (due || this.openedThisTick || atEnd)) {
      const frame = positionFrame(current.point.lat, current.point.lng, current.battery, current.signal);
      this.out.timed(elapsed, "send", `${frame}  along=${Math.round(current.alongM)}m`);
      if (!this.dry && this.link) {
        try {
          await this.link.send(frame);
        } catch (err) {
          this.out.timed(elapsed, "retry", err instanceof Error ? err.message : String(err));
          this.closeLink();
        }
      }
      if (this.socketUp || this.dry) {
        this.lastSend = elapsed;
      }
    }
    this.prev = current.effect;
  }

  private async syncBool(elapsed: number, want: boolean, which: "stay" | "public", msgType: string): Promise<void> {
    const armed = which === "stay" ? this.stayOn : this.publicOn;
    if (want === armed || !this.socketUp) {
      return;
    }
    const frame = boolFrame(msgType, want);
    this.out.timed(elapsed, "send", frame);
    if (!this.dry && this.link) {
      try {
        await this.link.send(frame);
      } catch (err) {
        this.out.timed(elapsed, "retry", err instanceof Error ? err.message : String(err));
        this.closeLink();
        return;
      }
    }
    if (which === "stay") {
      this.stayOn = want;
    } else {
      this.publicOn = want;
    }
  }

  private async open(elapsed: number): Promise<void> {
    let label = this.endpoint;
    if (this.openedOnce) {
      label = `nối lại ${this.endpoint}`;
    }
    if (this.dry) {
      this.socketUp = true;
      this.everConnected = true;
      this.openedOnce = true;
      this.openedThisTick = true;
      this.out.timed(elapsed, "dial", label);
      return;
    }
    const ws = await dialSocket(this.endpoint, this.token, this.signal).catch((err: unknown) => {
      const message = err instanceof Error ? err.message : String(err);
      throw new Error(`${this.endpoint}: ${message}`);
    });
    if (this.link) {
      this.link.close();
    }
    const started = this.start;
    const next = new Link(
      ws,
      (raw) => {
        if (this.out.quiet && messageType(raw) === "tracking.positions") {
          return;
        }
        this.out.timed((Date.now() - started) / 1000, "recv", compactJSON(raw));
      },
      (text) => {
        this.out.timed((Date.now() - started) / 1000, "recv", `socket đóng: ${text}`);
      },
    );
    this.link = next;
    this.socketUp = true;
    this.everConnected = true;
    this.openedOnce = true;
    this.openedThisTick = true;
    this.out.timed(elapsed, "dial", label);
  }

  private closeLink(): void {
    if (this.link) {
      this.link.close();
      this.link = null;
    }
    this.socketUp = false;
  }

  private async shutdown(elapsed: number, reason: string): Promise<void> {
    await this.finishBool(elapsed, "stay", "tracking.stayMode", "stayMode còn bật trên server vì socket đang đóng");
    await this.finishBool(
      elapsed,
      "public",
      "tracking.userStayPublic",
      "userStayPublic còn true trên server vì socket đang đóng",
    );
    this.closeLink();
    this.out.timed(elapsed, "stop", reason);
  }

  private async finishBool(elapsed: number, which: "stay" | "public", msgType: string, stuck: string): Promise<void> {
    const armed = which === "stay" ? this.stayOn : this.publicOn;
    if (!armed) {
      return;
    }
    if (this.dry || this.socketUp) {
      const frame = boolFrame(msgType, false);
      this.out.timed(elapsed, "send", frame);
      if (!this.dry && this.link) {
        try {
          await this.link.send(frame);
        } catch {
          // The process is already stopping.
        }
      }
    } else {
      this.out.timed(elapsed, "event", stuck);
    }
    if (which === "stay") {
      this.stayOn = false;
    } else {
      this.publicOn = false;
    }
  }
}

export async function run(
  sc: Scenario,
  route: Route,
  opt: Options,
  write: Writer,
  signal: AbortSignal,
): Promise<void> {
  const out = new Printer(write, opt.quiet);
  printPlan(out, sc, route, opt.dry);
  const runner = new Runner(sc, route, out, opt.dry, signal);
  await runner.run();
}

function printPlan(out: Printer, sc: Scenario, route: Route, dry: boolean): void {
  out.raw(`kịch bản  ${sc.name || "(không tên)"}`);
  if (sc.description) {
    out.raw(`mô tả     ${sc.description}`);
  }
  out.raw(`server    ${sc.server}`);
  out.raw(`room      ${sc.roomId || "(chưa đặt)"}`);
  out.raw(`auth      ${authLabel(sc)}`);
  out.raw(`đường     ${route.source} · ${route.points.length} điểm · ${Math.round(route.length)} m`);
  const repeat = sc.motion.loop ? "lặp lại polyline" : "một lượt, rồi đứng ở điểm cuối";
  out.raw(
    `chạy      ${sc.motion.speedMps.toFixed(1)} m/s · nhịp ${sc.motion.sendEveryMs} ms · ${repeat}`,
  );
  out.raw(`pin/sóng  ${sc.motion.batteryPercent}% · ${sc.motion.signalDbm} dBm`);
  if (sc.events.length === 0) {
    out.raw("sự kiện   không có");
  } else {
    out.raw(`sự kiện   ${sc.events.length}`);
    for (const event of sc.events) {
      const at = event.at.toFixed(1).padStart(6);
      const duration = event.duration.toFixed(1).padStart(5);
      out.raw(`  ${at}s  ${duration}s  ${event.name.padEnd(16)}  ${describe(event)}`);
      if (event.note) {
        out.raw(`${" ".repeat(37)}${event.note}`);
      }
    }
  }
  if (dry) {
    out.raw("chế độ    dry-run, không mở socket");
  }
  out.raw("");
}

function sleep(seconds: number, signal: AbortSignal): Promise<void> {
  const ms = Math.max(0, seconds) * 1000;
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new Error("aborted"));
      return;
    }
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(timer);
      reject(new Error("aborted"));
    };
    signal.addEventListener("abort", onAbort, { once: true });
  });
}
