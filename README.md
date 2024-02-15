# Usage

Run `npm start` to develop.

Run `./deploy.sh` to deploy (zero downtime deploy with Caddy).

# Infrastructure

- Single Go binary with embedded assets
- PM2 for process management
- Caddy as a reverse proxy
- Zero downtime deployment thanks to `lb_try_duration` and `lb_try_interval`