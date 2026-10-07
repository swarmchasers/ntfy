FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build && mv build/index.html build/app.html && rm build/config.js

FROM golang:1.26-bookworm AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build /src/server/site
RUN mkdir -p server/docs && printf 'See https://docs.ntfy.sh/\n' > server/docs/index.html
RUN CGO_ENABLED=1 go build -tags sqlite_omit_load_extension,osusergo,netgo -o /ntfy .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
COPY --from=go /ntfy /usr/local/bin/ntfy
ENTRYPOINT ["ntfy"]
