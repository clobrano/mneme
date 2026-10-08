# mneme

A terminal-native spaced-repetition tool for [atomic notes](https://github.com/clobrano/note-atomic). It reads your notes directory, generates flashcards deterministically — no LLM required — and schedules review sessions with [FSRS](https://github.com/open-spaced-repetition/go-fsrs).

Your notes are the source of truth and are never modified.

## How it works

Each atomic note follows a rigid format: one fact per paragraph, every sentence begins with the note's topic as subject, wiki-links mark related concepts. That rigidity is what makes offline card generation viable.

Given a note like:

```markdown
# Kubernetes
#research
#area/kubernetes

Kubernetes is a container orchestration platform.

Kubernetes uses [[etcd]] for persistent storage.

Kubernetes supports horizontal scaling automatically.
```

`mneme sync` generates:

| Kind       | Front                                           | Back                                              |
|------------|-------------------------------------------------|---------------------------------------------------|
| definition | What is Kubernetes?                             | Kubernetes is a container orchestration platform. |
| cloze      | Kubernetes uses ___ for persistent storage.     | Kubernetes uses [[etcd]] for persistent storage.  |
| property   | Kubernetes supports ___                         | Kubernetes supports horizontal scaling automatically. |

Learning history survives note edits. When you reword a fact the old card is **orphaned** (not deleted) and a new card is created. You then manually decide — via `mneme resolve` — whether to transfer the old card's study history to the new one.

## Install

**From source** (requires Go 1.22+):

```sh
git clone https://github.com/clobrano/mneme
cd mneme
make build          # produces ./mneme
sudo mv mneme /usr/local/bin/
```

**With `go install`:**

```sh
go install github.com/clobrano/mneme/cmd/mneme@latest
```

## Configuration

`mneme` runs with no config file — built-in defaults are used. To customise, create `~/.config/mneme/config.yaml`:

```yaml
# Where your atomic notes live. Never written to.
notes_dir: ~/Documents/notes

# Default filter applied by `review` when no --tag flags are given.
review:
  tags: [area/kubernetes, area/networking]  # empty → all due cards
  match: any                                # any (union) | all (intersection)
  exclude: [archive]                        # takes precedence over inclusion
```

Override the config file location with `--config PATH`.

**Path expansion:** both `~` and environment variables are expanded in `notes_dir` and `database`.

**XDG paths:**
- Config: `$XDG_CONFIG_HOME/mneme/config.yaml` (default `~/.config/mneme/config.yaml`)
- Database: `$XDG_STATE_HOME/mneme/mneme.db` (default `~/.local/state/mneme/mneme.db`)

## Usage

### Sync notes

Index your notes directory and detect card changes:

```sh
mneme sync
# 12 unchanged, 3 new
# 1 unchanged, 0 new, 2 orphaned — run 'mneme resolve'
```

Watch for changes continuously:

```sh
mneme sync --watch
```

### Review due cards

```sh
mneme review
```

Filter by tag:

```sh
mneme review --tag area/kubernetes
mneme review --tag area/kubernetes --tag area/networking        # union (default)
mneme review --tag area/kubernetes --tag area/networking --match all  # intersection
mneme review --tag area/kubernetes --exclude-tag archive
mneme review --note "Kubernetes" --limit 20
```

When no `--tag` flags are given, the defaults from `review:` in your config apply. CLI flags fully replace config defaults — they do not merge.

The review session runs as a TUI when connected to a terminal. Use `--no-tui` for plain stdout (useful in scripts).

### Handle orphaned cards

After rewording a fact or moving it between notes, `sync` marks the old card as **orphaned**. List them:

```sh
mneme orphans
```

Resolve them — transfer history to the new card, discard, or skip:

```sh
mneme resolve
```

The resolver walks orphans one at a time. For each orphan you choose:

- **`r<n>y`** — remap: type `r`, then the candidate number, then `y` to confirm
- **`dy`** — discard: type `d`, then `y` to confirm
- **`s`** — skip (non-destructive; resurfaces next time)
- **`w`** — widen candidate list to recently-created cards across all notes
- **`/keyword`** — filter candidates by keyword in note title or sentence

> **Enter alone does nothing.** Destructive actions require a deliberate confirmation key (`y`), so a reflexive keypress can never resolve anything.

Cross-note remap works: if you moved a fact from note A to note B, widen the candidate list and pick the new card in note B as the target.

Undo the last resolution batch:

```sh
mneme resolve --undo
```

### Stats

```sh
mneme stats
```

### Generate cards without a database

Preview the cards that would be generated from your notes, without writing anything to the database:

```sh
mneme generate
mneme generate --json
```

### JSON output

All commands (except `review`) accept `--json` for machine-readable output:

```sh
mneme --json sync
mneme --json stats
mneme --json orphans
mneme --json generate
```

## Card generation rules

| Source sentence | Card kind | Front | Back |
|---|---|---|---|
| First sentence containing `is` or `are` | `definition` | `What is <TOPIC>?` | full sentence |
| Any sentence with wiki-links | `cloze` — one card per distinct link | sentence with that link replaced by `___` | full sentence |
| Sentence with neither `is`/`are` nor wiki-links | `property` | sentence with predicate replaced by `___` | full sentence |

If the `is`/`are` sentence also contains wiki-links, both a definition card **and** cloze cards are generated.

Generation is fully deterministic and offline. The same note always produces the same cards.

## Identity model

Card identity is `(note_id, sentence_hash, cloze_key)`. The `sentence_hash` is computed by:

1. Trimming whitespace
2. Replacing `[[Link]]` with `Link` (brackets dropped)
3. Lowercasing
4. Collapsing whitespace runs to a single space
5. Stripping leading/trailing `.` and `,`
6. `hex(sha256(result))`

This means **cosmetic edits** — changing wiki-link capitalisation (`[[etcd]]` → `[[ETCD]]`), fixing trailing punctuation — produce the **same hash**, so the card is unchanged and keeps its full review history with no action required.

A meaningful reword produces a new hash → a new card. The old card is orphaned and awaits manual resolution.

**Learning history is never moved automatically.** `sync` only classifies cards. Only `resolve` moves history, and only on explicit user action.

## Building

```sh
make build   # go build -o mneme ./cmd/mneme
make test    # go test ./...
make lint    # go vet ./...
```

## License

MIT
