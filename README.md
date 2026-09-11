# WOK

This is application for personal use (1 person - me).

This is a project when looking for a new job. Simple application with list of applications.
This application is build for keeping track of CV sent (different CV with different information) - don't want to overlap with two different CV sent.

Applications contains:
- name of company
- aliases for company
- date of CV sent
- CV version (versioning by date)
- screenshots of job description
- link to job description

Applications list can be searched by company name or alias.
Some companies have red status - means I cannot apply there (or dont want to).\

Also each application has a status - sent, declined - no response, declined - with comment, in progress, accepted.

If company doesn't contact in 1 month - status is chenged to declined - no response. In progress

## Security

As this application will be placed on production with globasl access IP blocking will be provided.


## Tech stack
Application written in Go, with frontend in gohtml. Database simple noSQL for keeping information about company.


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

## Roadmap





7. stats + view dla pobranych
9. the protocol scrapper
