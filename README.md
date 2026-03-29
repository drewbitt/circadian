# Circadian

Self-hosted circadian rhythm tracker with energy schedule predictions, sleep debt tracking, and timed notifications.

**One Go binary. One SQLite file.**

## Features

- **FIPS Three Process Model** — scientifically-grounded alertness prediction (homeostatic pressure + circadian rhythm + ultradian rhythm + sleep inertia)
- **14-day weighted sleep debt** — tracks cumulative deficit with recency weighting
- **Energy zone classification** — morning peak, afternoon dip, evening peak, wind-down, melatonin window
- **Smart notifications** via [ntfy](https://ntfy.sh) — caffeine cutoff, energy dip, focus windows, bedtime reminders
- **5 data sources**: manual entry, Fitbit OAuth2, Android Health Connect, Apple Health XML, Gadgetbridge SQLite
- **Dark-themed dashboard** with real-time energy curve (Chart.js + Datastar SSE)
- Built on [PocketBase](https://pocketbase.io) — embedded auth, admin UI, SQLite, cron

## Running Locally

Requires [mise](https://mise.jdx.dev/) for tool management and task running.

```bash
# Install tools (Go, templ, air) and download Tailwind binary
mise install
mise run setup

# Start dev servers (templ watch + tailwind watch + air hot reload)
mise run dev
```

Visit `http://localhost:8090` for the app and `http://localhost:8090/_/` for the PocketBase admin panel. Create a superuser account on first run via the admin panel, then create your user account.

### Other commands

```bash
mise run test           # run all tests
mise run test:engine    # run engine tests only
mise run build          # production binary → ./circadian
mise run vet            # go vet
mise run fmt            # go fmt
mise run generate       # regenerate templ + tailwind
mise run clean          # remove build artifacts
mise tasks              # list everything
```

### Production (without Docker)

```bash
mise run build
./circadian serve --http=0.0.0.0:8090
```

Data is stored in `pb_data/` (SQLite + uploads). Back up this directory.

## Docker

```bash
docker build -t circadian .
docker run -d -p 8090:8090 -v circadian_data:/pb_data circadian
```

## Data Sources

| Source | Method | Auto-sync |
|--------|--------|-----------|
| Manual | Web form | — |
| Fitbit | OAuth2 | Every 4h |
| Health Connect | JSON file upload | Manual |
| Apple Health | ZIP/XML file upload | Manual |
| Gadgetbridge | SQLite file upload | Manual |

## Architecture

- **Backend**: PocketBase (Go) — auth, cron, SQLite, admin UI
- **Frontend**: Datastar + Templ + Tailwind CSS — server-rendered reactive UI
- **Engine**: FIPS Three Process Model ported to Go (~250 lines)
- **Notifications**: ntfy (single HTTP POST per notification)

## Configuration

All settings are per-user via the Settings page:

- **Sleep need** (default: 8h)
- **ntfy topic/server** for push notifications
- **Fitbit** OAuth2 connection
- **File imports** for Health Connect, Apple Health, Gadgetbridge

## License

MIT
