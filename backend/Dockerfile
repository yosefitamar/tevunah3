FROM surnet/alpine-wkhtmltopdf:3.20.0-0.12.6-full AS wkhtmltopdf

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/server  ./cmd/server  \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && go build -trimpath -ldflags="-s -w" -o /out/admin   ./cmd/admin

FROM alpine:3.20

RUN apk add --no-cache \
        ca-certificates tzdata \
        libstdc++ libx11 libxrender libxext libssl3 \
        fontconfig freetype ttf-dejavu ttf-liberation

COPY --from=wkhtmltopdf /bin/wkhtmltopdf /usr/local/bin/wkhtmltopdf
COPY --from=wkhtmltopdf /bin/libwkhtmltox* /usr/lib/

COPY --from=builder /out/server  /usr/local/bin/server
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/admin   /usr/local/bin/admin

RUN addgroup -S app && adduser -S -G app app \
 && mkdir -p /var/lib/tevunah/photos \
 && chown -R app:app /var/lib/tevunah

USER app
WORKDIR /app

EXPOSE 8080

CMD ["server"]
