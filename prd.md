# PRD — Atomic-Note Spaced-Repetition Tool

**Working name:** `notecards` (placeholder — rename freely)
**Audience:** an autonomous coding agent implementing the tool
**Status:** v1 scope frozen; post-v1 items listed separately

---

## 0. How to read this document

Requirement keywords **MUST**, **MUST NOT**, **SHOULD**, and **MAY** carry
RFC-2119 weight. Section 3 (Invariants) is non-negotiable and overrides any
convenience the implementation might otherwise reach for. When an invariant and
a feature appear to conflict, the invariant wins and the feature is descoped.

---

## 1. Summary

`notecards` reads a directory of Zettelkasten-style **atomic notes**, generates
spaced-repetition flashcards from them automatically, and runs study sessions
scheduled with FSRS. Notes are the source of truth and are never modified by the
tool. Cards are derived. Review history (the FSRS learning state) is durable and
survives edits to the underlying notes, but is **only ever reassigned by an
explicit, manual user action** — never by inference.

The atomic-note format is rigid (one fact per paragraph, every sentence begins
with the note's TOPIC as subject, permanent knowledge only, wiki-links mark
related concepts). That rigidity is what makes deterministic, offline card
generation viable without an LLM.

---

## 2. Goals and non-goals

### Goals

- Parse the atomic-note format deterministically and generate high-quality
  cards with **no LLM required**.
- Schedule and run review sessions using FSRS.
- Preserve learning state across note edits, splits, and merges.
- Give the user a **manual, explicit** way to reconnect an orphaned card's
  history to a regenerated card.
- Ship as a single dependency-light static binary, terminal-native.

### Non-goals (v1)

- **No GUI.** TUI and CLI only.
- **No note authoring or editing.** Writing notes is the job of the existing
  `note:atomic` skill; this tool is read-only over the notes directory.
- **No automatic remapping of learning state** under any confidence level
  (see §3). No similarity scoring that pre-selects a target.
- No cloud sync, server, or multi-device coordination.
- No Anki import/export.
- No concept-graph analytics (wiki-link graph traversal, cluster weakness) —
  post-v1.

---

## 3. Invariants (hard requirements)

These are the load-bearing rules. Implement them first; test them explicitly.

1. **No implicit remapping.** The tool **MUST NOT** reassign FSRS learning state
   from one card to another without a distinct, user-initiated action. There is
   no "high-confidence auto-remap." None.
2. **`sync` detects; it never resolves.** Reindexing **MUST** only classify
   cards (unchanged / new / orphaned) and report counts. It **MUST NOT** perform
   any remap, discard, or destructive action on orphans.
3. **The resolver is manual and explicit.** For each orphan the user **MUST**
   choose exactly one action. The tool **MUST NOT** pre-select any option. The
   confirming keystroke for a destructive action (remap, discard) **MUST** be
   distinct from Enter, so a reflexive keypress cannot resolve anything.
4. **Identity retention is not automation.** A regenerated card whose
   `sentence_hash` is byte-identical to an existing card **is** the same card;
   retaining its learning state is identity, not inference, and is the **only**
   automatic state retention permitted.
5. **Every destructive action is reversible.** Remap and discard **MUST** be
   undoable (`resolve --undo`) via `resolution_log` and soft-deletes. Cards
   carrying learning history **MUST NOT** be hard-deleted.
6. **Notes are read-only.** The tool **MUST NOT** write to, move, or rename any
   file under the notes directory.
7. **Deterministic generation works fully offline.** The LLM backend **MUST** be
   optional; the tool **MUST** produce a usable deck with the LLM disabled.

---

## 4. Note format contract

The tool parses notes matching the `note:atomic` skill's output. One note per
file at `<notes_dir>/<TOPIC>.md`.

Structure:

```
# <TOPIC>
#research
#study

<TOPIC> is <definition>. May contain [[wiki-links]].

<TOPIC> <verb> <permanent fact>.

<TOPIC> <verb> <permanent fact>.
```

Parsing rules:

- **Title:** first line matching `^#\s+(.+)$` → `TOPIC`.
- **Tags:** tag lines in the header block (before the first fact sentence)
  carry note-level metadata. A tag line matches `^(#[\w][\w/-]*\s*)+$` — one or
  more tags, so both `#research` on its own line and `#area/kubernetes #cka` on
  one line are valid. Hierarchical tags (`#area/kubernetes`) and hyphens are
  allowed. Tags do **not** affect card generation, but they **are** parsed,
  stored per note, and usable to filter review sessions (§9). Note: the
  `note:atomic` skill emits `#research` and `#study` on every note, so those two
  are universal and won't discriminate — filtering is useful once notes carry
  their own topic tags beyond that pair.
- **Fact sentences:** each remaining non-empty block, separated by blank lines,
  is exactly one sentence. Trim surrounding whitespace.
- Every fact sentence begins with `TOPIC` as subject (format guarantee). If a
  sentence does not, the tool **SHOULD** still process it and emit a warning.
- **Wiki-links** use `[[Concept Name]]`. A link's text is the target note's
  title (acronym form preferred, e.g. `[[CNV]]`). Wiki-link **target
  resolution** (link → file) is only needed post-v1 for the concept graph; in
  v1 links matter as cloze targets.

---

## 5. Card generation

### 5.1 Deterministic templates (default, no LLM)

For each note, generate cards by these rules, in order:

| Source sentence | Card kind | Front | Back |
|---|---|---|---|
| The `is` / `are` sentence | `definition` | `What is <TOPIC>?` | full sentence |
| Any sentence containing wiki-link(s) | `cloze` — **one card per distinct link** | sentence with that link's text replaced by `___` | full sentence |
| A sentence with no `is` and no wiki-links | `property` | sentence with the predicate (everything after `<TOPIC> <first-token>`) replaced by `___` | full sentence |

Notes:

- The `is` sentence, if it also contains wiki-links, produces **both** the
  `definition` Q&A card and one `cloze` card per link.
- `property` cards are the weakest deterministic output; the LLM backend adds
  the most value here. They **MAY** be gated behind a config flag if noisy.
- Generation output is stable: the same note **MUST** always yield the same set
  of `(kind, cloze_key, front, back)` tuples.

### 5.2 LLM backend (optional)

- Enabled per-invocation (e.g. `sync --llm`). Never on by default.
- Takes a fact sentence plus note context; returns 1–3 improved cards.
- LLM-sourced cards flow through the **same** identity, status, and resolution
  pipeline as template cards (tracked via `cards.source = 'llm'`).
- With the LLM disabled, behaviour is identical minus the LLM cards.

---

## 6. Identity model

Two distinct identities, deliberately separated:

- **Content identity** — `(note_id, sentence_hash, cloze_key)`. Describes the
  card as generated. Changes when the fact is reworded.
- **Learning identity** — the FSRS state (stability, difficulty, due, reps,
  lapses, history). Should survive rewording, splitting, and merging.

A **remap** is the manual assertion that a new content identity inherits an
existing learning identity.

### 6.1 `sentence_hash` normalization

Compute over the fact sentence:

1. Trim.
2. Replace `[[X]]` with its inner text `X` (drop the brackets, keep the words).
3. Lowercase.
4. Collapse internal whitespace runs to a single space.
5. Strip leading and trailing punctuation (`.`, `,`).
6. `sentence_hash = hex(sha256(result))`.

This makes trivial edits (link rename, whitespace, casing, trailing period)
normalize to the **same** hash → the card is `unchanged` and keeps history with
no user action. A meaningful wording change produces a new hash → a new card,
which is correct: a changed fact is a new thing to learn.

### 6.2 `cloze_key`

For a `cloze` card hiding `[[Foo]]`, `cloze_key` is the normalized inner text
(`foo`). For `definition` and `property` cards, `cloze_key` is empty. This keeps
the two cards from a two-link sentence distinct under
`UNIQUE(note_id, sentence_hash, cloze_key)`.

---

## 7. Data model (SQLite)

Storage is a single SQLite database (pure-Go driver, no CGo) at an XDG state
path. The notes directory is untouched.

```sql
notes(
  id            INTEGER PRIMARY KEY,
  path          TEXT UNIQUE NOT NULL,
  title         TEXT NOT NULL,          -- TOPIC
  content_hash  TEXT NOT NULL,          -- whole-file hash, for incremental sync
  indexed_at    TIMESTAMP NOT NULL
);

-- note-level tags; cards inherit their note's tags for filtering
note_tags(
  note_id       INTEGER NOT NULL REFERENCES notes(id),
  tag           TEXT NOT NULL,          -- stored without leading '#', lowercased
  PRIMARY KEY (note_id, tag)
);
-- index tag for fast "notes carrying tag X" lookups
CREATE INDEX idx_note_tags_tag ON note_tags(tag);

cards(
  id            INTEGER PRIMARY KEY,
  note_id       INTEGER NOT NULL REFERENCES notes(id),
  sentence_hash TEXT NOT NULL,
  cloze_key     TEXT NOT NULL DEFAULT '',
  kind          TEXT NOT NULL,          -- definition | cloze | property
  front         TEXT NOT NULL,
  back          TEXT NOT NULL,
  status        TEXT NOT NULL,          -- active | orphaned | superseded
  source        TEXT NOT NULL,          -- template | llm
  created_at    TIMESTAMP NOT NULL,
  UNIQUE(note_id, sentence_hash, cloze_key)
);

-- FSRS learning state, 1:1 with a card that owns history
reviews(
  card_id       INTEGER PRIMARY KEY REFERENCES cards(id),
  stability     REAL, difficulty REAL,
  due           TIMESTAMP,
  state         TEXT,                   -- new | learning | review | relearning
  reps          INTEGER NOT NULL DEFAULT 0,
  lapses        INTEGER NOT NULL DEFAULT 0,
  last_review   TIMESTAMP
);

review_log(
  id            INTEGER PRIMARY KEY,
  card_id       INTEGER NOT NULL REFERENCES cards(id),
  grade         INTEGER NOT NULL,       -- 1 again | 2 hard | 3 good | 4 easy
  reviewed_at   TIMESTAMP NOT NULL
  -- MAY snapshot scheduling params for later FSRS retuning
);

resolution_log(
  id             INTEGER PRIMARY KEY,
  batch_id       TEXT NOT NULL,         -- groups one resolve session, for undo
  action         TEXT NOT NULL,         -- remap | discard | skip
  orphan_card_id INTEGER NOT NULL,
  target_card_id INTEGER,               -- null for discard/skip
  created_at     TIMESTAMP NOT NULL
);
```

### 7.1 Remap operation (transaction)

Given orphan `O` and user-named target `T`:

1. Delete `T`'s freshly-generated blank `reviews` row (if any).
2. Repoint `reviews.card_id` and `review_log.card_id` from `O` to `T`.
3. Set `O.status = 'superseded'` (soft delete; **do not** `DELETE`).
4. Insert a `resolution_log` row under the current `batch_id`.

`resolve --undo` replays the last batch in reverse from `resolution_log`.

---

## 8. Card lifecycle

- **generation:** a new content identity with no prior match → `cards.status =
  active`, fresh `reviews` row (`state = new`).
- **unchanged:** regenerated card matches an existing active card by
  `(note_id, sentence_hash, cloze_key)` → keep as-is, history intact, no action.
- **orphaned:** an active card whose sentence no longer exists after reindex →
  `status = orphaned`. Excluded from study. Awaits manual resolution.
- **superseded:** an orphan whose history was remapped away, or that the user
  discarded → soft-deleted, retained for undo.

`sync` moves cards *into* `orphaned`. Only `resolve` moves them out.

---

## 9. CLI surface

All commands accept a global `--json` flag for machine/editor consumption
(the future Nvim frontend depends on this).

- `notecards sync [--watch] [--llm]`
  Reindex changed notes (incremental via `content_hash`), regenerate cards,
  refresh each changed note's `note_tags` rows (delete + reinsert), classify
  unchanged/new/orphaned, print a summary
  (`3 new, 2 orphaned — run 'notecards resolve'`). **Never resolves.**
- `notecards review [--tag T ...] [--match any|all] [--exclude-tag T ...] [--note N] [--limit K]`
  Study due cards. `--tag` is **repeatable** and restricts the session to cards
  belonging to notes carrying the given tag(s). `--match any` (default) includes
  a note if it has **any** listed tag (union); `--match all` requires it to have
  **all** listed tags (intersection). `--exclude-tag` (repeatable, **MAY**)
  drops notes carrying that tag, and takes precedence over inclusion. When no
  tag flags are given, the default filter comes from `review.tags` / `match` /
  `exclude` in the config file (§11.1); if those are also empty, all due cards
  are eligible. Tags are compared case-insensitively,
  without the leading `#`. Show front, reveal on keypress, grade
  again/hard/good/easy → feed FSRS → persist. Logs to `review_log`.
- `notecards orphans`
  List pending orphaned cards with their history (reps, lapses, current
  interval) so the stakes are visible.
- `notecards resolve`
  Manual interactive resolver (§10).
- `notecards resolve --undo`
  Revert the most recent resolve batch.
- `notecards stats`
  Deck counts, due-today, due forecast, orphan count.

---

## 10. The resolver (manual)

`resolve` walks pending orphans one at a time. For each orphan it displays the
orphan sentence and its learning history, then a candidate list of active cards
to remap onto. The user picks **exactly one** action:

- **Remap** — user selects a target card explicitly; §7.1 runs.
- **Discard** — orphan's history is dropped permanently (card → `superseded`);
  still undoable.
- **Skip** — orphan stays `orphaned` and pending; resurfaces next `resolve`.

Requirements:

- **No option is pre-selected.** Enter alone **MUST NOT** trigger remap or
  discard. Destructive actions use distinct, deliberate keys.
- **Candidate scope:** default to active cards in the **same note** (usually
  short). Keys widen to recently-created cards, or search by keyword / pick by
  note. This is what makes **cross-note remap** work: a fact moved from note A
  to note B orphans in A and appears new in B; the user searches for it in B and
  names it as the target.
- **No similarity scoring in v1.** The candidate list is plain and unordered by
  relevance — the tool offers no opinion about which target is "right." (Post-v1
  a similarity score **MAY** be offered strictly as an opt-in **sort key** that
  reorders the list; it **MUST NOT** ever pre-select or act.)

---

## 11. Architecture

- **Engine core (Go library):** parse → generate → schedule → store. All logic
  lives here and is unit-tested independently of any frontend.
- **CLI/TUI frontend (v1):** thin layer over the core. TUI via
  `charmbracelet/bubbletea` + `lipgloss`.
- **Nvim frontend (post-v1):** thin Lua plugin shelling out to the binary with
  `--json`; killer feature is jump-to-source-note during review. The core is
  identical for both frontends — only the presentation differs.

### Dependencies (keep light)

- `modernc.org/sqlite` — pure-Go SQLite, no CGo.
- `open-spaced-repetition/go-fsrs` — scheduling.
- `charmbracelet/bubbletea`, `lipgloss` — TUI (Phase 4).
- `gopkg.in/yaml.v3` — config parsing.
- Single static binary. DB at an XDG state path
  (`$XDG_STATE_HOME/<app>/`, default `~/.local/state/<app>/`).

### 11.1 Configuration

A YAML config file at `$XDG_CONFIG_HOME/<app>/config.yaml` (default
`~/.config/<app>/config.yaml`). A global `--config PATH` flag overrides the
location. The tool **MUST** run with no config file present, falling back to
built-in defaults; a missing file is not an error.

```yaml
# ~/.config/<app>/config.yaml

# Where the atomic notes live. Source of truth; never written to.
notes_dir: ~/Documents/RedHatNotes/Resources

# Default filter applied by `review` when no --tag flags are given.
review:
  tags: [area/kubernetes, area/networking]  # empty/absent → all due cards
  match: any                                # any (union) | all (intersection)
  exclude: [archive]                        # optional; takes precedence

# Optional override; defaults to the XDG state path above.
# database: ~/.local/state/<app>/<app>.db
```

Rules:

- **Precedence:** CLI flag > config value > built-in default, evaluated
  per-setting. `review --tag foo` fully replaces `review.tags` from config; it
  does not merge.
- Paths **MUST** expand a leading `~` and environment variables.
- `notes_dir` default, if unset everywhere, is
  `~/Documents/RedHatNotes/Resources`.
- Unknown keys **SHOULD** produce a warning, not a hard failure.
- The tool **MAY** provide `notecards config path` (print the resolved config
  path) and `notecards config init` (write a commented default file).

---

## 12. Build phases

**v1 = Phases 1–4.** The manual resolver is in v1.

1. **Parser + deterministic generator.** Load config (§11.1) to resolve
   `notes_dir`. Output cards as `--json`. No DB. Proves the format-to-card
   premise cheaply.
2. **SQLite + FSRS + `review`.** Plain stdout review loop. Now a usable SRS.
3. **`sync` (incremental) + orphan detection + manual `resolve` (CLI prompt) +
   `orphans` + `resolve --undo`.** Delivers the identity/resolution contract.
4. **Bubbletea TUI.** Review session, orphan resolver screen with an
   old-vs-new sentence diff, stats.
5. **Post-v1.** Nvim plugin; optional LLM backend; optional similarity sort-key
   (opt-in, never pre-selecting); concept-graph analytics.

---

## 13. Acceptance criteria

Each **MUST** be covered by an automated test.

1. **Deterministic generation.** A note with an `is` sentence and a fact
   sentence containing two wiki-links yields: 1 `definition` card + 2 `cloze`
   cards (one per link). Regenerating the same note yields the identical set.
2. **Cosmetic edit preserves history.** Renaming a wiki-link or changing
   trailing punctuation (normalizes to the same `sentence_hash`) → card stays
   `unchanged`, learning state intact, **0 orphans**.
3. **Fact edit orphans, never auto-moves.** Rewording a fact → old card
   `orphaned`, new card created `active` with fresh state, history **not**
   moved. Orphan appears in `orphans` and `resolve`.
4. **`sync` never resolves.** After any `sync`, no `resolution_log` rows exist
   and no learning state changed owner.
5. **Manual remap works and is reversible.** In `resolve`, remapping the orphan
   onto a named target transfers reps/lapses/due to the target; orphan →
   `superseded`; `resolve --undo` restores the prior state exactly.
6. **Reflexive-keypress safety.** Pressing Enter repeatedly in `resolve`
   resolves nothing.
7. **Cross-note remap.** A fact moved from note A to note B is resolvable by
   selecting B's new card as the target from within `resolve`.
8. **FSRS scheduling.** Grading `again` increments `lapses` and shortens the
   interval; grading `good` advances `due` per FSRS.
9. **Note deletion.** Deleting a note file orphans its cards (does not delete
   them); they are resolvable.
10. **Offline.** With the LLM disabled, the full pipeline (sync → review →
    resolve) works end to end.
11. **Tag filtering.** Given notes tagged `#area/kubernetes` and
    `#area/networking`: `review --tag area/kubernetes` yields only cards from
    the kubernetes note; `--tag area/kubernetes --tag area/networking` yields
    both (default `any`); `--match all` on two tags yields only notes carrying
    both; `--exclude-tag area/networking` drops the networking note even when it
    also matches an included tag. Editing a note's tags and re-running `sync`
    updates its `note_tags` rows without disturbing any card's learning state.
12. **Configuration.** With no config file, the tool runs on built-in defaults
    (default `notes_dir`, no tag filter). With a config file, `notes_dir` and
    the `review` defaults are honored; a `~`- or env-prefixed `notes_dir`
    expands correctly; and `review --tag X` on the CLI overrides (does not merge
    with) `review.tags` from config.

---

## 14. Open decisions (for the implementer to confirm with the user)

- **TUI-first vs Nvim-first** for the interactive frontend. Engine is identical;
  affects only whether Phase 4 or Phase 5 ships first.
- Whether `property` cards ship on by default or behind a config flag in v1.

