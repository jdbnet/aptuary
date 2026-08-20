# Aptuary

Single-binary APT repository server with GPG-signed metadata, dual HTTP listeners (admin dashboard + public repo), and API-key CI uploads.

## Features

- **Signed APT repos**: Generates `Packages`, `Release`, `InRelease`, and `Release.gpg`
- **Multi-distribution**: Configure multiple distros, components, and architectures
- **Admin dashboard**: Vue + Tailwind UI on a private port
- **Public endpoint**: Serves `pool/` and `dists/`, plus upload API for CI
- **API keys**: Scoped tokens (`apk_...`) for GitHub Actions and other automation

## Quick start

```bash
./build.sh
mkdir -p data
./aptuary docs/config.example.yaml
```

- Admin UI: http://127.0.0.1:8080 (default)
- Public repo: http://0.0.0.0:9090 (default)

First run creates an admin user (`admin` / `changeme`, or set `APTUARY_ADMIN_USER` and `APTUARY_ADMIN_PASSWORD`).

## Upload a package (CI)

Create an API key with `packages:write` in the dashboard, then:

```bash
curl -f -X POST \
  -H "Authorization: Bearer apk_your_key_here" \
  -F "file=@myapp_1.0.0_amd64.deb" \
  "https://apt.example.com/api/v1/upload?distribution=stable&component=main"
```

### GitHub Actions (this repo)

Releases use [GoReleaser](.goreleaser.yaml) for cross-compiles, `.deb` packaging, and GitHub Releases. The workflow uploads to Aptuary with inline `curl` (no repo scripts).

Repository secrets:

| Secret | Example |
|--------|---------|
| `APTUARY_URL` | `https://apt.example.com` |
| `APTUARY_API_KEY` | `apk_...` (scope: `packages:write`) |

Optional workflow env vars: `APTUARY_DISTRIBUTION`, `APTUARY_COMPONENT` (default `stable` / `main`).

Local snapshot build:

```bash
goreleaser build --snapshot --clean
```

### GitHub Actions (other projects)

```yaml
- name: Upload to Aptuary
  env:
    APTUARY_API_KEY: ${{ secrets.APTUARY_API_KEY }}
    APTUARY_URL: https://apt.example.com
  run: |
    curl -f -X POST \
      -H "Authorization: Bearer ${APTUARY_API_KEY}" \
      -F "file=@dist/myapp_${VERSION}_amd64.deb" \
      "${APTUARY_URL}/api/v1/upload?distribution=stable&component=main"
```

## Client setup

**Quick install** (per distribution; apt picks the right architecture):

```bash
curl -fsSL https://apt.example.com/install/stable.sh | sudo bash
```

Optional component filter: `.../install/stable.sh?component=main`

**Public endpoints:**

| URL | Purpose |
|-----|---------|
| `/aptuary.gpg` | GPG public key (armored) |
| `/install/{distribution}` | Client setup script (`/install/stable` or `/install/stable.sh`) |

Manual setup: see [docs/sources.list.example](docs/sources.list.example).

## Configuration

Environment overrides:

| Variable | Description |
|----------|-------------|
| `APTUARY_DATA_DIR` | Data directory |
| `APTUARY_ADMIN_LISTEN` | Admin bind address |
| `APTUARY_PUBLIC_LISTEN` | Public bind address |
| `APTUARY_PUBLIC_URL` | URL shown in sources.list snippets |
| `APTUARY_ADMIN_USER` | Bootstrap admin username |
| `APTUARY_ADMIN_PASSWORD` | Bootstrap admin password |

## systemd

```bash
sudo cp docs/aptuary.service /etc/systemd/system/
sudo cp docs/config.example.yaml /etc/aptuary/config.yaml
sudo useradd -r -s /usr/sbin/nologin aptuary || true
sudo mkdir -p /var/lib/aptuary
sudo chown aptuary:aptuary /var/lib/aptuary
sudo cp aptuary /usr/local/bin/aptuary
sudo systemctl enable --now aptuary
```

## Development

```bash
cd ui && npm install && npm run dev   # Vite on :3000, proxies API to :8080
go run ./cmd/aptuary ./docs/config.example.yaml
goreleaser build --snapshot --clean  # local release artifacts in dist/
```

## Requirements

- Go 1.23+
- Node 20+ (UI build)
- `gpg` on PATH for signing
- [GoReleaser](https://goreleaser.com/) for release builds (CI and local snapshots)
