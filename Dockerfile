FROM golang:1.26.4 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0: tắt cgo, tức là Go không link với code C (libc)
RUN CGO_ENABLED=0 GOOS=linux go build -o /Road-To-Destination-BE .

# Dùng alpine để chép các file binary đã build ở stage trên
FROM alpine:3.20
WORKDIR /app
COPY --from=build /Road-To-Destination-BE /app/Road-To-Destination-BE
COPY --from=build /app/docs /app/docs
CMD ["/app/Road-To-Destination-BE","server"]
