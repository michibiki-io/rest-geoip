# rest-geoip

[![Go Report Card](https://goreportcard.com/badge/github.com/TwistTheNeil/rest-geoip)](https://goreportcard.com/report/github.com/TwistTheNeil/rest-geoip)
[![main build](https://github.com/TwistTheNeil/rest-geoip/actions/workflows/docker-build-latest.yml/badge.svg?branch=main)](https://github.com/TwistTheNeil/rest-geoip/actions/workflows/docker-build-latest.yml)
[![main docker image](https://github.com/TwistTheNeil/rest-geoip/actions/workflows/docker-publish-tags.yml/badge.svg)](https://github.com/TwistTheNeil/rest-geoip/actions/workflows/docker-publish-tags.yml)

***For any current or previous stable versions, look in the appropriate tag's branch. [`main`](https://github.com/TwistTheNeil/rest-geoip) will be in continuous development***

A self hosted geoip lookup application written in Go and Vue.js 3 which provides a client with information about their IP address or any other. It uses the [Maxmind](https://www.maxmind.com) GeoLite2-City database.

This branch targets `Go 1.26.x` and the container build uses Docker BuildKit / `docker buildx`.

The webapp provides general geoip information. There is also an api available

```
GET  /                    : Return client IP Address (when used with curl or HTTPie)
GET  /api/geoip           : Return client Geoip information
GET  /api/geoip/:address  : Return Geoip information for "address"
GET  /api/geoip/cc/:address : Return ISO country code for "address"
PUT  /api/update          : Update the Maxmind database
```

The application doesn't provide a database. A `PUT` request to `/api/update` will update the database and will ideally be protected by an api key (header: `X-API-KEY`). If `GOIP_PROGRAM__API_KEY` is not set, then the application will set one on startup and notify via STDOUT

### Screenshots of optional webapp
![screenshot](docs/screen.png)

### Building and running

#### using docker compose

```bash
$ docker compose up --build
```

#### using docker compose for dev

```bash
$ docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

#### via pnpm and go
```bash
$ cd frontend
$ corepack enable
$ pnpm install --frozen-lockfile
$ pnpm build
$ cd ../
$ go build .
```

#### via docker buildx

```bash
$ docker buildx build --load -t rest-geoip:local .
```

#### multiarch image build

```bash
$ docker buildx build \
    --builder multiarch \
    -t michibiki/rest-geoip:1.4.0 \
    --platform=linux/amd64,linux/arm64 \
    --provenance=mode=max \
    --sbom=true \
    --push \
    ./
```

The CI workflows now build `linux/amd64` and `linux/arm64` images the same way.

### Configuration

Environment variables now follow the nested `GOIP_*` naming used by Viper. Common settings:

```bash
GOIP_PROGRAM__ENABLE_WEB=true
GOIP_PROGRAM__ENABLE_LOGGING=false
GOIP_PROGRAM__LISTEN_ADDRESS=0.0.0.0
GOIP_PROGRAM__LISTEN_PORT=1323
GOIP_PROGRAM__RELEASE_MODE=true
GOIP_PROGRAM__API_KEY=replace_me
GOIP_PROGRAM__API_RATE_LIMIT=3
GOIP_PROGRAM__ADMIN_NOTICE=
GOIP_MAPTILER__TOKEN=replace_me
GOIP_MAXMIND__LICENSE_KEY=replace_me
GOIP_MAXMIND__DB_LOCATION=/opt/
GOIP_MAXMIND__DB_FILE_NAME=GeoLite2-City.mmdb
```

The Go binary embeds the Vue app and serves it when `GOIP_PROGRAM__ENABLE_WEB=true`. In release mode (`GOIP_PROGRAM__RELEASE_MODE=true`) it serves the embedded SPA. In development mode it proxies requests to the Vite dev server from `docker-compose.dev.yml`.
