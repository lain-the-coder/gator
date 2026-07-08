# gator

A command-line RSS feed aggregator written in Go, backed by PostgreSQL. `gator` lets multiple users register feeds, follow feeds added by others, and read a personalized, date-ordered timeline of posts — all from the terminal. A long-running background worker continuously scrapes followed feeds and stores their posts for later reading.

On the surface it's a terminal feed reader. Underneath it's a study in production backend patterns: a hand-rolled command router with middleware, a type-safe database layer built on migrations and generated queries, relational modeling (one-to-many and many-to-many), a concurrent background worker, and context-aware error handling that knows the difference between a mistake a user made and a failure the system needs to survive.

---

## Contents

- [What It Does](#what-it-does)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Commands](#commands)
- [Architecture](#architecture)
- [Design Highlights](#design-highlights)
- [Tech Stack](#tech-stack)

---

## What It Does

- **Register and switch users** — a local, multi-user CLI where the active user is tracked in a config file.
- **Add and follow feeds** — any user can add an RSS feed; other users can follow feeds without re-adding them.
- **Continuous background aggregation** — a long-running worker fetches the least-recently-scraped feed on a timer, parses its posts, and stores them, cycling fairly through all feeds.
- **Personalized reading** — the `browse` command shows a timeline of posts drawn only from the feeds the current user follows, newest first.

---

## Prerequisites

You'll need two things installed:

- **Go** (1.23+) — to install and build the CLI. [Install Go](https://go.dev/doc/install)
- **PostgreSQL** (15+) — the database `gator` stores feeds and posts in. [Install PostgreSQL](https://www.postgresql.org/download/)

Once Postgres is running, create a database for `gator`:

```bash
psql postgres
CREATE DATABASE gator;
\q
```

---

## Installation

Install the `gator` CLI directly with `go install`:

```bash
go install github.com/lain-the-coder/gator@latest
```

This compiles a statically-linked binary and places it on your `PATH` (typically `~/go/bin` — make sure that's on your `PATH`). After installation, `gator` runs as a standalone binary — no Go toolchain needed at runtime.

> Go programs compile to static binaries. `go run .` is for development; the installed `gator` command is the production tool.

Database schema is managed with migrations. With [Goose](https://github.com/pressly/goose) installed, from the project's `sql/schema` directory:

```bash
goose postgres "postgres://username:password@localhost:5432/gator" up
```

---

## Configuration

`gator` reads its configuration from a JSON file at `~/.gatorconfig.json`. Create it with your database connection string:

```json
{
  "db_url": "postgres://username:password@localhost:5432/gator?sslmode=disable"
}
```

Replace `username` and `password` with your Postgres credentials. The `?sslmode=disable` suffix is required for local, unencrypted connections. The `current_user_name` field is written automatically when you register or log in — you don't need to set it yourself.

---

## Usage

First, register a user (this also logs you in):

```bash
gator register alice
```

Add a feed and start following it:

```bash
gator addfeed "TechCrunch" "https://techcrunch.com/feed/"
```

In one terminal, start the aggregator running continuously (it scrapes immediately, then on each interval):

```bash
gator agg 1m
```

The interval is a Go duration string — `30s`, `1m`, `1h`. Be considerate of the servers you scrape; start slow and keep `Ctrl+C` ready.

In another terminal, read your personalized post timeline:

```bash
gator browse 5
```

---

## Commands

| Command     | Arguments      | Description                                                             |
| ----------- | -------------- | ----------------------------------------------------------------------- |
| `register`  | `<name>`       | Create a new user and log in as them                                    |
| `login`     | `<name>`       | Switch the active user to an existing one                               |
| `users`     | —              | List all users, marking the active one                                  |
| `addfeed`   | `<name> <url>` | Add a feed and auto-follow it (requires login)                          |
| `feeds`     | —              | List all feeds and who added them                                       |
| `follow`    | `<url>`        | Follow an existing feed (requires login)                                |
| `following` | —              | List the feeds the current user follows (requires login)                |
| `unfollow`  | `<url>`        | Unfollow a feed (requires login)                                        |
| `agg`       | `<interval>`   | Start the continuous scraping worker (e.g. `1m`)                        |
| `browse`    | `[limit]`      | Show recent posts from followed feeds; default limit 2 (requires login) |
| `reset`     | —              | Delete all users (and cascade to their data)                            |

---

## Architecture

`gator` is a one-shot CLI for most commands (run, do one thing, exit) with a single long-running exception (`agg`). Commands are dispatched through a hand-built router, and shared resources — the database connection pool and config — are injected into every handler rather than reached for globally.

```
        terminal input
              |
              v
     +-----------------+
     |  command router |   map[string]handler, no switch statements
     +--------+--------+
              |  injects *state (db pool + config)
              v
     +-----------------+
     |    middleware   |   middlewareLoggedIn: resolves + injects the current user
     +--------+--------+
              |
              v
     +-----------------+        +----------------------+
     | command handler |<------>|  PostgreSQL (sqlc)   |
     +-----------------+        +----------------------+
              ^                            ^
              |                            |
        +-----+------+              writes | posts
        | agg worker |--- fetch RSS -------+
        | (ticker)   |    (HTTP + XML parse)
        +------------+
```

The **scraping engine** (`agg`) and the **reading engine** (`browse`) are deliberately decoupled: the worker fills the `posts` store in the background, and users read a personalized slice of it on demand. They communicate only through the database.

**Data model** — five tables. `users` and `feeds` in a one-to-many relationship (a feed has a creator); `feed_follows` as a junction table giving users and feeds a many-to-many relationship (many users can follow many feeds); and `posts` linked to their source `feed`. Foreign keys use `ON DELETE CASCADE` throughout, so removing a user or feed cleans up all dependent records — no orphans.

---

## Design Highlights

**Middleware for cross-cutting auth.** Several commands require a logged-in user. Rather than repeating the user-lookup in each handler, a higher-order function `middlewareLoggedIn` wraps them — resolving the current user once, injecting it into the handler, and failing cleanly if no valid user is active. Handlers that need a user simply declare it in their signature and receive it pre-loaded.

**Type-safe database access without an ORM.** Queries are written as plain SQL and compiled into type-safe Go by [sqlc](https://sqlc.dev/) at build time — no runtime reflection, no ORM magic. Schema changes are versioned migrations applied with Goose. What runs at runtime is standard `database/sql` calls wrapped in generated, statically-typed methods.

**Fair, polite, resilient scraping.** The worker picks the least-recently-fetched feed each cycle (`ORDER BY last_fetched_at ASC NULLS FIRST`), marks it fetched _before_ the slow network call to prevent double-fetching, and spaces requests on a `time.Ticker` so it never floods a server. It absorbs per-feed failures — a single unreachable feed logs and is skipped, rather than crashing the loop.

**Context-aware error handling.** The same class of error is handled differently depending on where it occurs. A duplicate feed URL entered by a user fails fast and tells them; the identical unique-constraint violation in the background scraper (which happens every cycle, since feeds re-serve old posts) is silently skipped so the worker keeps running. Interactive commands fail loud; the unattended worker fails quiet and survives.

**Nullable data handled explicitly.** External RSS data is messy — missing descriptions, unparseable dates. Nullable columns are modeled with `sql.NullString`/`sql.NullTime` and checked for validity at both write and read time, so incomplete feed data degrades gracefully instead of crashing the pipeline.

---

## Tech Stack

- **Go** — CLI, concurrency (goroutines, `time.Ticker`), HTTP client, XML parsing
- **PostgreSQL** — relational storage with foreign keys, cascade deletes, and unique constraints
- **[sqlc](https://sqlc.dev/)** — compile-time generation of type-safe Go from raw SQL
- **[Goose](https://github.com/pressly/goose)** — versioned database migrations
- **[lib/pq](https://github.com/lib/pq)** — PostgreSQL driver
