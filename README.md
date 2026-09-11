# WOK

A personal job-search tracker for a single user. It keeps a record of every job application
and collects offers from job boards to show which technologies are in demand.

## Features

- Applications: company, date, whether there was a chat, salary range (offered and my own),
  stack, offer link, outcome, notes and a status (`applied`, `progress`, `failed`, `abandon`).
- Crawler: scrapes offers from justjoin.it and nofluffjobs.com for a fixed set of categories
  (Go, PHP, AI/ML, C/C++, Software Architect) into the `possibilities` table.
- Scraped offers list and technology stats per category.
- Database cleaner and a read-only MCP server (below).

## Security

The panel is exposed publicly but meant for one person:

- Login with a single user whose bcrypt hash is set in `LOGIN_PASS_HASH` (`make hashgen`
  generates one). Login attempts are limited to 5 per IP per 15 minutes.
- Sessions are stored in Redis (in memory as a fallback) and sent as an `HttpOnly`,
  `SameSite=Strict` cookie, marked `Secure` when served over HTTPS.
- Requests to common scanner paths (`/.env`, `/wp-*`, ...) and any unexpected
  `POST`/`PUT`/`DELETE` ban the client IP for 30 minutes.
- HTTPS through Let's Encrypt when `WEB_DOMAIN` is set (see Configuration).

## Tech stack

Go with server-side rendered `html/template` pages, Chi router, PostgreSQL through
`sqlc`-generated queries, Redis for sessions and IP bans, `chromedp` for the justjoin.it
crawler, Docker Compose for running the stack.

## Getting started

```
cp .env.default .env    # fill in LOGIN_USER and LOGIN_PASS_HASH (make hashgen)
make up                 # build and start web, crawler, redis and postgres
```

`docker/README.md` covers the images, deploy, backups and the other Makefile targets.

## Configuration

All settings come from environment variables (`.env` is loaded if present); `.env.default`
lists them.

| Variable | Purpose |
| --- | --- |
| `LOGIN_USER`, `LOGIN_PASS_HASH` | Panel login and bcrypt hash of its password |
| `WEB_ADDR` | Listen address without TLS, `:80` by default |
| `WEB_DOMAIN` | Domain for HTTPS; when set, the panel serves HTTPS on `:443` with a Let's Encrypt certificate and `:80` only answers ACME challenges and redirects to HTTPS |
| `REDIS_ADDR` | Redis address; without Redis sessions and bans are kept in memory |
| `POSTGRES_*` | Postgres connection |
| `SENTRY_DSN` | Optional Sentry error reporting |

With `WEB_DOMAIN` set, the domain's DNS has to point at the server and ports 80 and 443 have
to be reachable for the certificate to be issued; `docker-compose.yaml` publishes only `80`,
so add `"443:443"` to the `web` ports first. Certificates are cached in the
`web_certs` volume.


## MCP server

`cmd/mcp` hands the database to an MCP client - Claude Code, for one - as seven read-only
tools: `list_categories`, `list_applications`, `get_application`, `list_possibilities`,
`get_possibility`, `list_duplicates` and `tech_stats`. There is no arbitrary-SQL tool: the
only statements that can run are the sqlc queries behind those seven, and the pool connects with
`default_transaction_read_only=on` on top of that.

It talks stdio and the client starts it over ssh, so nothing listens publicly and the ssh
key is the only credential involved.

### Deploy

It is the `mcp` service of the Compose stack, in the `tools` profile so `make up` does not
start it: every client session gets its own throwaway container from
`docker compose run -T mcp`, on the Compose network next to Postgres. `make deploy` builds
its image along with the rest, so there is nothing to deploy separately.

### Register with Claude Code

```
claude mcp add wok-db -s user -- ssh wok-vps 'cd /opt/wok && exec docker compose run --rm -T mcp'
```

`cd` is needed because `docker compose` looks for `docker-compose.yaml` (and the `.env` it
fills the Postgres credentials from) in the working directory, and `ssh host cmd` starts in
`$HOME`. `-T` keeps a terminal off the session: stdin and stdout have to be the raw JSON-RPC
stream. The ssh user has to be able to run docker without a password - root, or a member of
the `docker` group; `sudo` would ask for one and hang the session.

An `~/.ssh/config` entry keeps the command short and the session from idling out:

```
Host wok-vps
    HostName <ip>
    User <user>
    IdentityFile ~/.ssh/id_ed25519
    ServerAliveInterval 30
```

### Check it by hand

```
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tech_stats","arguments":{"category":"go"}}}' \
| ssh wok-vps 'cd /opt/wok && docker compose run --rm -T mcp'
```

Two JSON-RPC responses come back on stdout and nothing else - every log line, the server's
and Compose's own, goes to stderr, because anything else on stdout corrupts the protocol.

## Database cleaner

`cmd/cleaner` runs once and exits. It repairs possibilities whose location or tech the
crawler used to store as raw nofluff JSON, turning them into the comma separated lists it
stores today, and then brings every tech list to the normalized form the crawlers store
(`technology.Normalize`: lowercase, JavaScript and TypeScript grouped as
`typescript/javascript`, no duplicates). Rows already in shape are left alone, so it is safe
to run again.

Last it marks repeated offers - the same company asking for the same technologies in one
category, like EPAM posting one offer for many cities. The oldest offer keeps its tech; every
later copy gets `duplicate` as its tech and `duplicate_of` pointing at the kept offer. Copies
are not deleted, since the next crawl would only bring them back; the crawler never rewrites
the tech of an offer it already has, and each run marks the copies scraped since. Stats count
the copies under `duplicate`, so they show how many repeated offers come in. The MCP
`list_duplicates` tool shows what got grouped, to check the rule does not catch different
jobs.

It is the `cleaner` service of the Compose stack, in the `tools` profile so `make up` does
not start it. `make cleaner` runs it once in a throwaway container of the image the last
`make deploy` built, so it runs the deployed code:

```
cd /opt/wok && make cleaner
```

Take a `make db-backup` first - the rewrite cannot be undone. From root's crontab (so
docker needs no sudo), e.g. every night at 3:

```
0 3 * * * cd /opt/wok && make cleaner >> /var/log/wok-cleaner.log 2>&1
```

## License

MIT, see `LICENSE`.
