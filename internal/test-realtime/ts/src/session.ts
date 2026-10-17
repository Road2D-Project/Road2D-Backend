// Login and the lobby socket. The JWT goes on Authorization, the same way a native client sends it.

import { request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";

import WebSocket, { type RawData } from "ws";

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function roomURL(server: string, roomID: string): string {
  let parsed: URL;
  try {
    parsed = new URL(server.trim());
  } catch {
    throw new Error(`server không hợp lệ: ${server}`);
  }
  if (!parsed.host) {
    throw new Error(`server không hợp lệ: ${server}`);
  }
  if (parsed.protocol === "http:") {
    parsed.protocol = "ws:";
  } else if (parsed.protocol === "https:") {
    parsed.protocol = "wss:";
  } else if (parsed.protocol !== "ws:" && parsed.protocol !== "wss:") {
    throw new Error(`server cần http, https, ws hoặc wss: ${server}`);
  }
  const base = parsed.pathname.replace(/\/$/, "");
  parsed.pathname = `${base}/v1/realtime/rooms/${encodeURIComponent(roomID)}`;
  parsed.search = "";
  parsed.hash = "";
  return parsed.toString();
}

export function requireRoom(roomID: string): void {
  if (!roomID) {
    throw new Error("cần roomId (cờ -room hoặc field roomId), là UUID của trip");
  }
  if (!uuidPattern.test(roomID)) {
    throw new Error(`roomId phải là UUID của trip, nhận ${JSON.stringify(roomID)}`);
  }
}

export async function login(server: string, username: string, password: string, signal: AbortSignal): Promise<string> {
  if (!username || !password) {
    throw new Error("cần accessToken hoặc cả username và password");
  }
  const endpoint = `${server.replace(/\/+$/, "")}/v1/auth/user/login`;
  const body = await postJSON(endpoint, { username, password }, signal);
  const parsed = body.json as { data?: { access_token?: string } };
  if (body.status !== 200) {
    throw new Error(`đăng nhập ${body.statusText}: ${body.text}`);
  }
  const token = parsed.data?.access_token ?? "";
  if (!token) {
    throw new Error("đăng nhập không trả access_token");
  }
  return token;
}

interface HTTPResult {
  status: number;
  statusText: string;
  text: string;
  json: unknown;
}

function postJSON(endpoint: string, payload: unknown, signal: AbortSignal): Promise<HTTPResult> {
  const url = new URL(endpoint);
  const data = Buffer.from(JSON.stringify(payload));
  const request = url.protocol === "https:" ? httpsRequest : httpRequest;
  return new Promise((resolve, reject) => {
    const req = request(
      url,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": data.length,
        },
        timeout: 20_000,
      },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (chunk: Buffer) => chunks.push(chunk));
        res.on("end", () => {
          const text = Buffer.concat(chunks).toString("utf8").trim().slice(0, 1024);
          let json: unknown = {};
          try {
            json = text ? JSON.parse(text) : {};
          } catch {
            json = {};
          }
          resolve({ status: res.statusCode ?? 0, statusText: String(res.statusCode ?? ""), text, json });
        });
      },
    );
    const abort = () => {
      req.destroy(new Error("aborted"));
    };
    if (signal.aborted) {
      abort();
    } else {
      signal.addEventListener("abort", abort, { once: true });
    }
    req.on("error", (err) => reject(new Error(`đăng nhập: ${err.message}`)));
    req.on("timeout", () => req.destroy(new Error("timeout")));
    req.write(data);
    req.end();
  });
}

export function dialSocket(endpoint: string, token: string, signal: AbortSignal): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    let settled = false;
    const fail = (err: Error) => {
      if (settled) {
        return;
      }
      settled = true;
      reject(err);
    };
    const ws = new WebSocket(endpoint, {
      headers: { Authorization: `Bearer ${token}` },
      handshakeTimeout: 10_000,
    });
    const onAbort = () => {
      ws.close();
      fail(new Error("aborted"));
    };
    signal.addEventListener("abort", onAbort, { once: true });
    ws.once("open", () => {
      signal.removeEventListener("abort", onAbort);
      if (settled) {
        ws.close();
        return;
      }
      settled = true;
      resolve(ws);
    });
    ws.on("unexpected-response", (_req, res) => {
      const chunks: Buffer[] = [];
      res.on("data", (chunk: Buffer) => chunks.push(chunk));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8").trim().slice(0, 512);
        fail(new Error(`websocket ${res.statusCode}: ${text || res.statusMessage || ""}`));
      });
    });
    ws.on("error", (err) => {
      const message = err.message;
      if (message.startsWith("Unexpected server response")) {
        return;
      }
      fail(new Error(`websocket: ${message}`));
    });
  });
}

export class Link {
  dead = false;
  quietClose = false;
  private reported = false;

  constructor(
    readonly ws: WebSocket,
    private readonly onFrame: (raw: string) => void,
    private readonly onDown: (text: string) => void,
  ) {
    ws.on("message", (data) => this.onFrame(rawText(data)));
    ws.on("error", (err) => this.fail(err.message));
    ws.on("close", () => this.fail("đóng"));
  }

  send(frame: string): Promise<void> {
    if (this.dead) {
      return Promise.reject(new Error("socket chưa mở"));
    }
    return new Promise((resolve, reject) => {
      this.ws.send(frame, (err) => {
        if (err) {
          reject(err);
        } else {
          resolve();
        }
      });
    });
  }

  close(): void {
    this.quietClose = true;
    this.ws.close();
  }

  private fail(text: string): void {
    this.dead = true;
    if (this.quietClose || this.reported) {
      return;
    }
    this.reported = true;
    this.onDown(text);
  }
}

function rawText(data: RawData): string {
  if (Array.isArray(data)) {
    return Buffer.concat(data).toString("utf8");
  }
  if (Buffer.isBuffer(data)) {
    return data.toString("utf8");
  }
  return Buffer.from(data).toString("utf8");
}

export function messageType(raw: string): string {
  try {
    const head = JSON.parse(raw) as { type?: unknown };
    return typeof head.type === "string" ? head.type : "";
  } catch {
    return "";
  }
}

export function compactJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw));
  } catch {
    return raw;
  }
}

export function positionFrame(lat: number, lng: number, battery: number, signal: number): string {
  return `{"type":"tracking.position","payload":{"lat":${lat.toFixed(6)},"lng":${lng.toFixed(6)},"batteryPercent":${battery},"signalDbm":${signal}}}`;
}

export function boolFrame(msgType: string, value: boolean): string {
  return `{"type":${JSON.stringify(msgType)},"payload":${value}}`;
}
