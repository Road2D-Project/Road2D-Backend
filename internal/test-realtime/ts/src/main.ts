// CLI for the TypeScript mock rider. The Go client in ../go follows the same script.

import { loadScenario, normalize, scenarioRoute } from "./scenario.js";
import { run } from "./run.js";

function main(): void {
  const args = parseArgs(process.argv.slice(2));
  if (args.help === true || !stringArg(args.file)) {
    process.stderr.write(`${usage()}\n`);
    process.exit(args.help === true ? 0 : 2);
  }

  let sc;
  try {
    sc = loadScenario(stringArg(args.file));
    applyOverrides(sc, args);
    normalize(sc);
  } catch (err) {
    process.stderr.write(`${err instanceof Error ? err.message : err}\n`);
    process.exit(1);
  }

  let route;
  try {
    route = scenarioRoute(sc);
  } catch (err) {
    process.stderr.write(`${err instanceof Error ? err.message : err}\n`);
    process.exit(1);
  }

  const ac = new AbortController();
  process.on("SIGINT", () => ac.abort());
  run(sc, route, { dry: args["dry-run"] === true, quiet: args.quiet === true }, (line) => console.log(line), ac.signal)
    .then(() => process.exit(0))
    .catch((err: unknown) => {
      process.stderr.write(`${err instanceof Error ? err.message : err}\n`);
      process.exit(1);
    });
}

function applyOverrides(sc: ReturnType<typeof loadScenario>, args: Args): void {
  sc.server = first(stringArg(args.server), process.env.MOCK_SERVER, sc.server);
  sc.roomId = first(stringArg(args.room), process.env.MOCK_ROOM_ID, sc.roomId);
  sc.auth.accessToken = first(stringArg(args.token), process.env.MOCK_ACCESS_TOKEN, sc.auth.accessToken);
  sc.auth.username = first(stringArg(args.username), process.env.MOCK_USERNAME, sc.auth.username);
  sc.auth.password = first(stringArg(args.password), process.env.MOCK_PASSWORD, sc.auth.password);
}

function first(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (value && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

type Args = Record<string, string | boolean>;

function parseArgs(argv: string[]): Args {
  const out: Args = {};
  for (let i = 0; i < argv.length; i++) {
    let arg = argv[i];
    if (!arg.startsWith("-")) {
      continue;
    }
    arg = arg.startsWith("--") ? arg.slice(2) : arg.slice(1);
    const eq = arg.indexOf("=");
    if (eq >= 0) {
      out[arg.slice(0, eq)] = arg.slice(eq + 1);
      continue;
    }
    const next = argv[i + 1];
    if (next === undefined || next.startsWith("-")) {
      out[arg] = true;
    } else {
      out[arg] = next;
      i++;
    }
  }
  return out;
}

function stringArg(value: string | boolean | undefined): string {
  return typeof value === "string" ? value : "";
}

function usage(): string {
  return `Giả lập một rider trên websocket tracking.

  npx tsx src/main.ts --file ..\\scenario.home.json --dry-run
  npx tsx src/main.ts --file ..\\scenario.home.json --room <trip-uuid> --token <jwt>
  npx tsx src/main.ts --file ..\\scenario.home.json --room <trip-uuid> --username demo --password secret

Cờ ghi đè file. Biến môi trường (khi cờ bỏ trống): MOCK_SERVER, MOCK_ROOM_ID,
MOCK_ACCESS_TOKEN, MOCK_USERNAME, MOCK_PASSWORD.

  --file       file JSON kịch bản
  --server     ghi đè server, ví dụ http://127.0.0.1:8080
  --room       ghi đè roomId, UUID của trip
  --token      access token, bỏ qua đăng nhập
  --username   username nếu không có token
  --password   password đăng nhập
  --dry-run    in kịch bản và frame sẽ gửi, không mở socket
  --quiet      không in tracking.positions từ server
  --help       in trợ giúp`;
}

main();
