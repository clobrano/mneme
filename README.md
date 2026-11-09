# mneme

A terminal-native spaced-repetition tool for [atomic notes](https://github.com/clobrano/note-atomic). It reads your notes directory, generates flashcards deterministically — no LLM required — and schedules review sessions with [FSRS](https://github.com/open-spaced-repetition/go-fsrs).

Your notes are the source of truth and are never modified.

## How it works

Each atomic note follows a simple format: one fact per paragraph, every sentence begins with the note's topic as subject. You mark **what to remember** with two light markers, so mneme never has to guess which part of a sentence is the answer:

- `**bold**` — the bold words become the blank of a cloze card (one card per bold span).
- `::` — splits the sentence into cue (before) and answer (after).

Given a note like:

```markdown
# Kubernetes
#research
#area/kubernetes

Kubernetes is :: a container orchestration platform.

Kubernetes stores its state in **[[etcd]]**.

Kubernetes can be configured via :: `kubectl apply -f`.

Which port does the Kubernetes API server use? :: `6443`

Kubernetes supports horizontal scaling.
```

`mneme sync` generates:

| Kind       | Front                                            | Back                                              |
|------------|--------------------------------------------------|---------------------------------------------------|
| definition | What is Kubernetes?                              | Kubernetes is a container orchestration platform. |
| cloze      | Kubernetes stores its state in ___.              | Kubernetes stores its state in [[etcd]].          |
| cloze      | Kubernetes can be configured via ___.            | Kubernetes can be configured via `kubectl apply -f`. |
| question   | Which port does the Kubernetes API server use?   | `6443`                                            |

The last fact has no marker, so it produces no card — not every fact has to be studied.

Learning history survives note edits. When you reword a fact the old card is **orphaned** (not deleted) and a new card is created. You then manually decide — via `mneme resolve` — whether to transfer the old card's study history to the new one.

## Writing notes with an AI agent

The repository ships a Claude Code skill, [`atomic-note`](skills/atomic-note/SKILL.md), that writes an atomic note in this format for a given topic, marks the answers, and lists the cards it will produce.

```
/atomic-note etcd
```

The topic is required on every call. The destination directory (`NOTES_ROOT`, default `~/Notes`; keep it equal to mneme's `notes_dir`) and the header tags (`TAGS`, default `#mneme`) are set once, in the skill's configuration block.

Install it by copying it into your Claude Code skills directory, then edit the configuration block in the copy:

```sh
mkdir -p ~/.claude/skills
cp -r skills/atomic-note ~/.claude/skills/
```

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
notes_dir: ~/Notes

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

### Preview cards without a database

Preview the cards your notes would produce, without writing anything to the database. Use it while writing a note to check each card reads the way you intended:

```sh
mneme generate
mneme generate --unmarked   # also list facts that produce no card
mneme --json generate
```

Facts with a malformed marker (two `::`, nothing before or after `::`) are reported as warnings.

### JSON output

All commands (except `review`) accept `--json` for machine-readable output:

```sh
mneme --json sync
mneme --json stats
mneme --json orphans
mneme --json generate
```

## Card generation rules

| Fact | Card kind | Front | Back |
|---|---|---|---|
| `… **X** …` | `cloze` — one card per bold span | sentence with `X` replaced by `___` (other bold spans shown as plain text) | full sentence |
| `cue :: answer` | `cloze` | `cue ___` | full sentence |
| `Question? :: answer` (cue ends with `?`) | `question` | the question | the answer |
| `<TOPIC> is :: …` / `<TOPIC> are :: …` (cue is exactly that) | `definition` | `What is <TOPIC>?` | full sentence |
| no `**` and no `::` | — no card | | |

Rules:

- At most one `::` per fact. A fact can combine `::` with bold spans; each produces its own card.
- Markers inside `` `code spans` `` are ignored, so `` `std::vector` `` or `` `**/*.go` `` are safe. Slashes, paths and URLs need no escaping.
- Markers are removed from the card text; wiki-links stay as written.
- Explicit questions (`… ? :: …`) are exempt from the topic-first rule.

Generation is fully deterministic and offline. The same note always produces the same cards.

## Identity model

Card identity is `(note_id, sentence_hash, cloze_key)`. The `sentence_hash` is computed by:

0. Removing the markers (`::` and the `**` around bold spans)
1. Trimming whitespace
2. Replacing `[[Link]]` with `Link` (brackets dropped)
3. Lowercasing
4. Collapsing whitespace runs to a single space
5. Stripping leading/trailing `.` and `,`
6. `hex(sha256(result))`

This means **cosmetic edits** — changing wiki-link capitalisation (`[[etcd]]` → `[[ETCD]]`), fixing trailing punctuation — produce the **same hash**, so the card is unchanged and keeps its full review history with no action required.

Moving a marker — choosing a different answer in the same sentence — keeps the `sentence_hash` but changes the `cloze_key` (the normalized answer), so it creates a new card and orphans the old one; `resolve` can transfer the history.

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
