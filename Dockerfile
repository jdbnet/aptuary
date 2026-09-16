FROM node:24-alpine AS web
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY ui/ ./
RUN npm run build

FROM golang:1.27-bookworm AS build
WORKDIR /src
ARG VERSION=dev
RUN apt-get update && apt-get install -y --no-install-recommends gcc && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN rm -rf ui/dist && mkdir -p ui/dist
COPY --from=web /ui/dist ./ui/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=${VERSION}" -o /aptuary ./cmd/aptuary

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates gnupg \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd -r -d /var/lib/aptuary -s /usr/sbin/nologin aptuary \
	&& mkdir -p /var/lib/aptuary \
	&& chown aptuary:aptuary /var/lib/aptuary

COPY --from=build /aptuary /usr/local/bin/aptuary

EXPOSE 8080 9090
USER aptuary
WORKDIR /var/lib/aptuary
ENTRYPOINT ["/usr/local/bin/aptuary"]
CMD ["/var/lib/aptuary/config.yaml"]
