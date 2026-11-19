# Giả lập realtime ở nhà

Một process là một rider. Client đọc file JSON, đi dọc một polyline, và làm các event đúng giây. Không có giao diện. Bản Go và bản TypeScript dùng cùng file, cùng giao thức, và in cùng một log khi `--dry-run`.

Thư mục này nằm trong repo để mở cùng backend. Server không import code ở đây. Key bản đồ để trong `config.js` (copy từ `config.example.js`); file đó không được commit. Mật khẩu trong file kịch bản cũng để trống khi đẩy lên git.

```
internal/test-realtime/
  README.md
  scenario_user_1.home.json    kịch bản cho client dòng lệnh
  index.html            trang bản đồ có sẵn, bấm để chạy tay
  go/                   client Go
  ts/                   client TypeScript
```

`index.html` nối socket bằng `?access_token=` (trình duyệt không đặt được header) và gửi `tracking.position` khi bấm chạy. Hai client dòng lệnh làm cùng việc đó từ file JSON, và thêm event theo giây. Chúng gửi token bằng header `Authorization: Bearer`, cách client native dùng. Cả hai cách server đều nhận.

## Ngồi ở nhà thì đang giả cái gì

App thật gửi GPS của điện thoại. Ở nhà không có quãng đường đó, nên file đóng vai điện thoại:

- `path.polyline` là đường điện thoại báo lên. Mã hóa giống polyline Goong mà server lưu (precision 1e5).
- `motion` là tốc độ và nhịp gửi `tracking.position`.
- `events` là việc xảy ra trên đồng hồ tường: mất mạng, đứng yên, lệch khỏi đường, pin yếu, sóng yếu, leader cho cả đoàn nghỉ.

Server ghi đúng `lat`, `lng`, pin và sóng client gửi. Khoảng mỗi 500ms phòng nhận `tracking.positions`. Phần snap trên server chỉ để biết rider đã tới đâu trên branch của trip. Tọa độ phát ra phòng là tọa độ client gửi.

Polyline trong `scenario_user_1.home.json` là đoạn khoảng 1729 m quanh quận 1, chỉ để có đường mà chạy. Muốn pin nằm trên đường của trip thì dán polyline của branch đó vào `path.polyline`.

Nếu điền `path.points` từ 2 điểm trở lên, client dùng points và bỏ qua polyline. Dưới 2 điểm thì decode polyline.

```json
"path": {
  "points": [
    { "lat": 10.7725, "lng": 106.6980 },
    { "lat": 10.7742, "lng": 106.7008 },
    { "lat": 10.7761, "lng": 106.7034 },
    { "lat": 10.7784, "lng": 106.7051 },
    { "lat": 10.7809, "lng": 106.7076 },
    { "lat": 10.7832, "lng": 106.7092 }
  ]
}
```

Sáu điểm trên chính là polyline mẫu.

## Vào phòng

Socket là `GET /v1/realtime/rooms/{tripUUID}`.

Trip phải đang active. User của token phải là member đang active. Sai thì HTTP trả lỗi và không có socket.

Client gửi `Authorization: Bearer <access token>`. Token lấy từ `POST /v1/auth/user/login`:

```json
{ "username": "demo", "password": "secret12" }
```

Đọc `data.access_token`. Username 4–16 ký tự, password 8–32 ký tự.

Một process là một user. Hai rider là hai terminal, hai tài khoản. Có thể cùng một polyline; đổi `at` nếu không muốn event của hai người rơi cùng lúc.

Server mặc định `http://127.0.0.1:8080`. Dùng `127.0.0.1` để khỏi đụng IPv6 của `localhost` trên Windows.

## Kiểm tra checkpoint và Redis worker

Với DB và Redis trên cloud, cấu hình `DB_URL` và `REDIS_URL` trong `.env` ở gốc
repo. Dùng `rediss://` khi Redis yêu cầu TLS. Server và worker phải dùng cùng
DB/schema, Redis database và cấu hình quota Goong. Không cần bật container DB
hay Redis local. Xem [hướng dẫn Redis Streams](../../docs/tracking-streams.md)
để cấu hình kết nối, kiểm tra stream/group và dọn dữ liệu test.

Chạy migration khi có thay đổi schema, rồi giữ server và worker ở hai terminal
riêng, từ thư mục gốc repo:

```powershell
go run . migrate
```

```powershell
go run . server
```

```powershell
go run . worker
```

Worker phải chạy liên tục để xử lý redirect. Stream và consumer group tự tạo
khi khởi động; không cần chạy worker một lần để chuẩn bị rồi tắt.
`GET /health` báo kết nối DB/Redis, không xác nhận worker đang xử lý được job.

Trước bài live, chọn tài khoản và trip dành cho test, kiểm tra user có assigned
branch và trip có stored travels. Dùng polyline của đúng hop từ
`GET /v1/trips/{tripId}/graph` cho `path.polyline`, đồng thời bỏ `path.points`
nếu muốn dùng polyline. Đường mẫu quận 1 có thể khác tuyến của trip và gây
off-route ngay cả khi chưa bật event `deviate`.

Từ terminal thứ ba, chạy client Go; các biến `MOCK_ROOM_ID`, `MOCK_USERNAME`
và `MOCK_PASSWORD` có thể ghi đè cấu hình JSON. Chúng phải có trong môi trường
của terminal chạy client; client không tự nạp `.env` của backend.

```powershell
cd internal/test-realtime/go
go run . -file ../scenario_user_1.home.json -quiet
```

Đối chiếu các bước sau trong log và dữ liệu lưu:

1. Khi join, nhận `tracking.success` và `tracking.snapshot`.
2. Khi đủ mẫu lệch tuyến, nhận `tracking.offRoute`, `tracking.life` nếu mất life,
   rồi `tracking.redirect` từ `pending` sang `ready` hoặc `failed`.
3. Event `disconnect` đóng socket rồi nối lại; snapshot giữ trạng thái hiện tại.
4. Đợi checkpoint được ghi vào PostgreSQL. Dừng đột ngột riêng tiến trình server
   test rồi chạy lại, giữ client và worker đang chạy. Ctrl+C server là shutdown
   có drain; muốn kiểm tra crash thì cần kill đúng tiến trình server test.
5. Client tự thử nối lại sau khi đã từng kết nối thành công. Snapshot sau recovery
   giữ lives/progress đã lưu, có `awaitingFreshPosition: true` và `observedAt` cũ.
   GPS mới cho phép tracking tiếp tục và checkpoint sequence tăng tiếp.
6. Khi job xử lý xong, kiểm tra `XPENDING`, group lag và `XLEN`; pending/lag về 0,
   còn stream có thể rỗng vì code ACK rồi XDEL entry.

Live simulation ghi checkpoint và kết quả redirect thật lên cloud, có thể trừ
lives và gọi Goong. Dùng Ctrl+C dừng client khi đủ bước; client live không tự kết
thúc sau event cuối. Bài một rider chỉ kiểm tra luồng, chưa đo khả năng chịu tải.
Test Go trong `module/tracking/worker` kiểm tra reclaim sau worker crash và tái
sử dụng result đã lưu; muốn xác minh tương tự trên cloud cần một bài kill worker
riêng khi job còn pending.

Xóa stream sau test theo [hướng dẫn cleanup](../../docs/tracking-streams.md#clean-up-after-a-test).
Việc đó giữ nguyên checkpoint và result trong PostgreSQL; user reconnect vẫn có
thể nhận snapshot cũ.

Access token hiện chỉ được kiểm tra khi mở socket. Socket đang mở không tự đóng
khi token hết hạn; reconnect bằng token cũ sẽ nhận HTTP 401. Cả hai client chưa
tự refresh token, nên lỗi này cần xử lý riêng khi chạy bài dài.

## Client gửi

```json
{"type":"tracking.position","payload":{"lat":10.772500,"lng":106.698000,"batteryPercent":84,"signalDbm":-74}}
{"type":"tracking.stayMode","payload":true}
{"type":"tracking.userStayPublic","payload":false}
```

`stayMode` và `userStayPublic` là boolean JSON, không phải object.

Server im lặng bỏ qua, không trả `system.error`, khi:

- `stayMode` không phải leader, hoặc trip không active lúc bật, hoặc trip không đang stay lúc tắt
- `userStayPublic` trong khi trip chưa ở stay

`signalDbm` được server đổi thành số vạch:

| dBm | vạch |
| --- | --- |
| từ -70 trở lên | 5 |
| từ -85 trở lên | 4 |
| từ -100 trở lên | 3 |
| từ -110 trở lên | 2 |
| từ -120 trở lên | 1 |
| thấp hơn | 0 |

Mặc định -74 dBm là 4 vạch. Event `weakSignal` trong file mẫu đặt -118, tức 1 vạch.

## Server gửi lại

Mỗi frame text là một dòng `recv`. Server ping mỗi 20 giây, client trả pong. Ping không được in.

| type | lúc nào | payload |
| --- | --- | --- |
| `system.presence` | vào phòng lần đầu, hoặc rời hẳn | `userId`, `online` |
| `tracking.success` | vừa vào | chuỗi `Go go`. Nối lại thì `Welcome back` |
| `tracking.snapshot` | join/reconnect hoặc gửi `tracking.resync` | `tripId`, `revision`, `status`, `riders`; gồm thời điểm quan sát, cờ chờ GPS mới và redirect nếu được phép thấy |
| `tracking.connection` | nối lại, sau khi đã từng vào | `userId`, `connectionStatus` = `RECONNECTING`. Các `tracking.positions` sau đó mang `CONNECTING` |
| `tracking.onLeave` | socket đóng | chuỗi |
| `stayMode` | leader đổi chế độ nghỉ | `isChangeMode`, `message` |
| `tracking.positions` | khoảng mỗi 500ms | mảng vị trí |
| `system.error` | type không có dấu chấm, hoặc namespace lạ | `code` |

`tracking.positions` dùng field camelCase theo JSON tag của struct phát ra:

```json
{
  "type": "tracking.positions",
  "payload": [
    {
      "userId": "…",
      "lat": 10.77,
      "lng": 106.7,
      "batteryPercent": 84,
      "signalBars": 4,
      "driveStatus": "OnROUTE",
      "connectionStatus": "CONNECTING"
    }
  ]
}
```

`driveStatus`: `OnROUTE`, `DEVIATED`, `STOP`, `EMERGENCY`, `StayPublic`, `StayPrivate`, `COMPLETED`.

`connectionStatus`: `CONNECTING`, `UNSTABLE`, `DISCONNECTED`, `RECONNECTING`.

Khi trip đang stay, server chỉ đưa vào mảng những rider đã `StayPublic`. Pin biến mất cho đến khi client gửi `userStayPublic` true. File mẫu bật public 5 giây sau `stayMode`.

Khi `disconnect`, server phát `system.presence` với `online: false` và `tracking.onLeave`. Rider vẫn có thể còn trong `tracking.positions` với `connectionStatus` `DISCONNECTED`, vì mất mạng không xóa người khỏi phòng.

`-quiet` / `--quiet` ẩn riêng `tracking.positions`, vì frame này đến khoảng hai lần một giây.

## File kịch bản

`at` là giây kích hoạt, tính từ lúc client bắt đầu chạy. `duration` là số giây event còn hiệu lực. Cửa sổ là `[at, at+duration)`: đúng giây `at` thì bật, đúng giây kết thúc thì tắt.

`disconnect`, `pause` và `stayMode` làm rider đứng yên trên polyline. Các event khác vẫn tiến. Hai khoảng đứng yên chồng nhau chỉ tính một lần. Hai event cùng tên chồng nhau thì dòng đứng sau trong file thắng phần số (dBm, pin, độ lệch).

Đồng hồ là giây tường. Nhịp gửi mặc định 1 giây, nhưng cạnh event vẫn đánh thức client giữa hai nhịp. `at: 25.5` tắt mạng tại 25.5, không đợi đến giây 26.

Dry-run chạy đến khi đi hết polyline và mọi event đã tắt, rồi thoát. Chạy thật giữ socket và đứng ở điểm cuối cho đến Ctrl+C. Ctrl+C gửi `stayMode false` và `userStayPublic false` nếu hai cờ đó còn bật, rồi đóng socket.

| name | việc client làm |
| --- | --- |
| `disconnect` | đóng socket, không gửi, đứng yên. Hết giờ thì nối lại và gửi tiếp từ đúng mét đang đứng |
| `pause` | socket vẫn mở, gửi lại cùng một điểm |
| `deviate` | lệch sang phải hướng đi. `offsetMeters` mặc định 80. Số âm là lệch trái |
| `weakSignal` | ghi đè `signalDbm`. Bỏ trống thì -120 |
| `lowBattery` | ghi đè phần trăm pin. Bỏ trống thì 5 |
| `stayMode` | gửi `tracking.stayMode` true khi vào, false khi ra. Đứng yên. Cần tài khoản leader |
| `userStayPublic` | gửi `tracking.userStayPublic` true khi vào, false khi ra. Nên nằm trong cửa sổ `stayMode` |

Khóa `event` dùng được thay cho `events`. `atSec` và `durationSec` dùng được thay cho `at` và `duration`. `name` không phân biệt hoa thường.

Bỏ trống thì `server` là `http://127.0.0.1:8080`, tốc độ 8 m/s, nhịp 1000 ms, pin 80%, sóng -74 dBm. Giá trị 0 ở bốn field đó cũng được hiểu là bỏ trống. `sendEveryMs` nhỏ nhất là 200.

`loop: true` thì hết polyline quay lại đầu. Dry-run khi loop dừng ở event cuối, hoặc sau 60 giây nếu không có event.

## Dòng lệnh

Cờ ghi đè file. Nếu cờ bỏ trống, tới biến môi trường, rồi tới file.

| cờ | biến môi trường |
| --- | --- |
| `-server` / `--server` | `MOCK_SERVER` |
| `-room` / `--room` | `MOCK_ROOM_ID` |
| `-token` / `--token` | `MOCK_ACCESS_TOKEN` |
| `-username` / `--username` | `MOCK_USERNAME` |
| `-password` / `--password` | `MOCK_PASSWORD` |

`-dry-run` in kế hoạch và từng frame sẽ gửi, không mở socket, không cần token. `-quiet` chỉ có tác dụng khi đã nối, để bớt `tracking.positions`.

Có token thì không đăng nhập. Không có token thì cần cả username và password. Token không được in ra log.

### Go

Cần Go 1.22 trở lên.

```powershell
cd internal\test-realtime\go
go run . -file ..\scenario_user_1.home.json -dry-run
go run . -file ..\scenario_user_1.home.json -room <trip-uuid> -token <jwt>
go run . -file ..\scenario_user_1.home.json -room <trip-uuid> -username demo -password secret12
```

`go test .` trong thư mục `go` kiểm tra polyline, khoảng đứng yên, và dry-run của file mẫu.

Nếu chạy lệnh với không tag thì sẽ mặt định chạy đăng nhập, tự set token.

### TypeScript

Cần Node 18 trở lên.

```powershell
cd internal\test-realtime\ts
npm install
npm run check
npx tsx src/main.ts --file ..\scenario_user_1.home.json --dry-run
npx tsx src/main.ts --file ..\scenario_user_1.home.json --room <trip-uuid> --token <jwt>
```

`npm run dry` là dry-run của file mẫu. `npm run check` kiểm tra polyline mẫu, không mở socket.

## Log

Đầu ra là kế hoạch, rồi từng nhịp. Đoạn dưới là dry-run của file mẫu, cắt bớt các giây chỉ gửi position:

```
kịch bản  home-ride
đường     polyline · 6 điểm · 1729 m
chạy      8.0 m/s · nhịp 1000 ms · một lượt, rồi đứng ở điểm cuối
     8.0s   12.0s  weakSignal        signalDbm=-118
    25.0s   50.0s  disconnect        đóng socket, đứng yên trên polyline

t=   0.0s  dial   (chưa có roomId)
t=   0.0s  send   {"type":"tracking.position","payload":{"lat":10.772500,"lng":106.698000,"batteryPercent":84,"signalDbm":-74}}  along=0m
t=   8.0s  event  weakSignal bật (12.0s) — signalDbm=-118 · along 64 m
t=  25.0s  event  disconnect bật (50.0s) — đóng socket, đứng yên trên polyline · along 200 m
t=  75.0s  event  disconnect tắt · along 200 m
t=  75.0s  dial   nối lại (chưa có roomId)
t= 150.0s  send   {"type":"tracking.stayMode","payload":true}
t= 155.0s  send   {"type":"tracking.userStayPublic","payload":true}
t= 306.1s  stop   hết dry-run
```

`along` là mét đã đi trên polyline. Trong 50 giây mất mạng, `along` đứng ở 200. Hết giờ, client nối lại và đi tiếp.

Khi đã có room và server, dòng `dial` là URL `ws://127.0.0.1:8080/v1/realtime/rooms/<trip-uuid>`. Dòng `recv` là frame server đẩy xuống. Kỳ vọng ngay sau khi nối: `tracking.success` với `Go go`, rồi `system.presence`, rồi `tracking.positions` khoảng mỗi nửa giây.
