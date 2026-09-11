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

`cmd/mcp` hands the database to an MCP client - Claude Code, for one - as six read-only
tools: `list_categories`, `list_applications`, `get_application`, `list_possibilities`,
`get_possibility` and `tech_stats`. There is no arbitrary-SQL tool: the only statements
that can run are the sqlc queries behind those six, and the pool connects with
`default_transaction_read_only=on` on top of that.

It talks stdio and the client starts it over ssh, so nothing listens publicly and the ssh
key is the only credential involved.

### Deploy

```
make mcp-deploy                                   # defaults to wok-vps:/opt/wok
make mcp-deploy WOK_SSH_HOST=root@1.2.3.4 WOK_REMOTE_DIR=/srv/wok
```

The binary is not part of the Compose stack, so a plain `scp` is the whole deploy. It runs
on the VPS host and reaches Postgres on the loopback, which is why the `postgres` service
publishes `127.0.0.1:5432`; `make deploy` has to have run once since that was added.

### Register with Claude Code

```
claude mcp add wok-db -s user -- ssh wok-vps 'cd /opt/wok && POSTGRES_HOST=127.0.0.1 exec bin/wok-mcp'
```

Both halves of that command matter. `cd` is needed because config comes from the `.env` in
the working directory and `ssh host cmd` starts in `$HOME`. `POSTGRES_HOST` has to be
overridden because `.env` names the Compose service, which does not resolve from the host;
godotenv leaves variables already set in the environment alone, so the override wins.

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
| ssh wok-vps 'cd /opt/wok && POSTGRES_HOST=127.0.0.1 ./bin/wok-mcp'
```

Two JSON-RPC responses come back on stdout and nothing else - every log line goes to
stderr, because anything else on stdout corrupts the protocol.

## Database cleaner

`cmd/cleaner` runs once and exits. It repairs possibilities whose location or tech the
crawler used to store as raw nofluff JSON, turning them into the comma separated lists it
stores today, and then brings every tech list to the normalized form the crawlers store
(`technology.Normalize`: lowercase, JavaScript and TypeScript grouped as
`typescript/javascript`, no duplicates). Rows already in shape are left alone, so it is safe
to run again.

It is a plain binary, not part of the Compose stack. The VPS has no Go toolchain, so it is
cross-compiled locally and shipped, then run on the VPS host against Postgres on the
loopback (the same `127.0.0.1:5432` the MCP server uses):

```
make cleaner-deploy               # locally, after every change to the cleaner
```

Take a `make db-backup` first - the rewrite cannot be undone. From cron, e.g. every night
at 3:

```
0 3 * * * cd /opt/wok && make cleaner >> /var/log/wok-cleaner.log 2>&1
```

## Roadmap





7. stats + view dla pobranych
9. the protocol scrapper
