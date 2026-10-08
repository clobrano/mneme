## Relevant Files

- `cmd/mneme/main.go` — Entry point; wires all CLI commands to the root command.
- `internal/config/config.go` — `Config` struct, YAML loading, XDG path resolution, `~` and `$ENV` expansion, built-in defaults.
- `internal/config/config_test.go` — Unit tests for config loading, path expansion, missing file, unknown key warning, CLI-vs-config precedence.
- `internal/parser/note.go` — `Note` struct and `ParseNote()` function: title, tags, fact sentences, wiki-links.
- `internal/parser/note_test.go` — Unit tests for all parser rules.
- `internal/generator/generator.go` — Deterministic card generation (definition / cloze / property), `sentence_hash`, `cloze_key`.
- `internal/generator/generator_test.go` — Unit tests for all card kinds, combined is+wiki-link case, hash normalization.
- `internal/store/db.go` — SQLite open/create at XDG state path; schema migration that creates all tables and indexes.
- `internal/store/notes.go` — `NoteRepository`: upsert, get by path, list all, mark deleted.
- `internal/store/tags.go` — `NoteTagRepository`: replace all tags for a note (delete + reinsert).
- `internal/store/cards.go` — `CardRepository`: upsert by `(note_id, sentence_hash, cloze_key)`, get by note, set status, list orphaned, list active.
- `internal/store/reviews.go` — `ReviewRepository` and `ReviewLogRepository`: create, get, update after grade, insert log entry.
- `internal/store/resolution.go` — `ResolutionLogRepository`: insert, get last batch, delete batch (for undo).
- `internal/store/store_test.go` — Integration tests against a real (in-memory) SQLite instance; verifies UNIQUE constraints and all repository operations.
- `internal/scheduler/fsrs.go` — Thin wrapper around `go-fsrs`: takes a grade (1–4) and current `reviews` row, returns updated row.
- `internal/scheduler/fsrs_test.go` — Unit tests: `again` increments lapses + shortens interval; `good` advances due.
- `internal/sync/sync.go` — File scanner, whole-file `content_hash`, incremental reindex, orphan detection, note-deletion handling. **Never writes to resolution_log.**
- `internal/sync/sync_test.go` — Tests: sync never resolves; cosmetic edit → 0 orphans; wording change → 1 orphan + 1 new card; note deletion → orphaned cards.
- `internal/cli/review.go` — `review` command: due-card query, tag/match/exclude/limit flags, plain-stdout review loop, config-default application.
- `internal/cli/sync.go` — `sync` command: calls sync engine, prints summary, supports `--watch`.
- `internal/cli/orphans.go` — `orphans` command: lists pending orphans with their review history.
- `internal/cli/resolve.go` — `resolve` command (plain CLI prompt version): walks orphans, shows candidates, enforces deliberate-key safety, remap/discard/skip, `--undo`.
- `internal/cli/stats.go` — `stats` command: active card count, due today, 7-day forecast, orphan count.
- `internal/tui/review.go` — Bubbletea model for the review session screen.
- `internal/tui/resolve.go` — Bubbletea model for the orphan resolver screen (includes old-vs-new diff).
- `internal/tui/stats.go` — Bubbletea model for the stats screen.

### Notes

- Run all tests with `go test ./...` from the project root.
- Run a single package's tests with `go test ./internal/parser/` (replace path as needed).
- The store integration tests use an in-memory SQLite database (`file::memory:?cache=shared`) — no temp files needed.
- The LLM backend is explicitly **out of scope for v1**; do not add `--llm` wiring beyond a stub flag that prints "not implemented."
- Invariants in §3 of the PRD are the highest-priority requirements. If any sub-task seems to conflict with them, the invariant wins and the task is descoped or adjusted.
- Build the binary with `go build -o mneme ./cmd/mneme`.

---

## Tasks

- [ ] 1.0 Project Setup & Module Scaffolding
  - [ ] 1.1 Run `go get` to add all required dependencies to `go.mod`: `modernc.org/sqlite`, `github.com/open-spaced-repetition/go-fsrs`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `gopkg.in/yaml.v3`. Pin the versions with `go mod tidy`.
  - [ ] 1.2 Create the directory tree: `cmd/mneme/`, `internal/config/`, `internal/parser/`, `internal/generator/`, `internal/store/`, `internal/scheduler/`, `internal/sync/`, `internal/cli/`, `internal/tui/`.
  - [ ] 1.3 Create `cmd/mneme/main.go` with a minimal `main()` that prints a usage line and exits cleanly — just enough to confirm `go build -o mneme ./cmd/mneme` works.
  - [ ] 1.4 Write a top-level `Makefile` with targets `build`, `test`, and `lint` (`go vet ./...`).
  - [ ] 1.5 Verify: `make build` produces the `mneme` binary with no errors.

- [ ] 2.0 Configuration Loading
  - [ ] 2.1 In `internal/config/config.go`, define the `Config` struct with fields: `NotesDir string`, `Database string`, and a nested `Review struct { Tags []string; Match string; Exclude []string }`. Match the YAML layout in §11.1 of the PRD exactly.
  - [ ] 2.2 Implement `DefaultConfig()` that returns built-in defaults: `NotesDir = ~/Documents/RedHatNotes/Resources`, `Review.Match = "any"`, all slices empty.
  - [ ] 2.3 Implement `XDGConfigPath()` that returns `$XDG_CONFIG_HOME/mneme/config.yaml`, falling back to `~/.config/mneme/config.yaml` when `XDG_CONFIG_HOME` is unset.
  - [ ] 2.4 Implement `XDGStatePath()` that returns `$XDG_STATE_HOME/mneme/mneme.db`, falling back to `~/.local/state/mneme/mneme.db`.
  - [ ] 2.5 Implement `Load(path string) (*Config, error)`: read and unmarshal the YAML file; if the file does not exist, return `DefaultConfig()` with no error; if the file exists but has unknown keys, print a warning to stderr and continue.
  - [ ] 2.6 Implement `ExpandPath(p string) string` that expands a leading `~` to `$HOME` and then calls `os.ExpandEnv`. Call this on `NotesDir` and `Database` after loading.
  - [ ] 2.7 In `main.go`, add a global `--config PATH` flag; when set, use that path instead of `XDGConfigPath()`. Pass the resolved `*Config` into all commands.
  - [ ] 2.8 Write `internal/config/config_test.go` covering: missing file returns defaults; known keys load correctly; unknown key produces a warning but no error; `ExpandPath` expands `~` and `$HOME`; CLI `--config` overrides XDG path (acceptance criterion 12).

- [ ] 3.0 Note Parser
  - [ ] 3.1 In `internal/parser/note.go`, define `type Note struct { Title string; Tags []string; Facts []string; FilePath string }`.
  - [ ] 3.2 Implement `ParseNote(path string) (*Note, error)`: open and read the file, then apply all rules below. Return an error only for unreadable files — never for format issues.
  - [ ] 3.3 Extract title: scan lines for the first match of `^#\s+(.+)$`; capture group 1 is `Note.Title` (= TOPIC).
  - [ ] 3.4 Extract tags from the header block (lines before the first blank line that follows the title): match each line against `^(#[\w][\w/-]*(\s+#[\w][\w/-]*)*)\s*$`; split on whitespace; strip the leading `#`; lowercase; append to `Note.Tags`. Both `#research` alone and `#area/kubernetes #cka` on one line are valid.
  - [ ] 3.5 Split the remainder (after the header block) into fact sentences: split on one or more blank lines (`\n\n+`); trim each block; discard empty results; store in `Note.Facts`.
  - [ ] 3.6 After parsing, iterate facts: if a fact does not start with `Note.Title` (case-insensitive), print a warning to stderr (`"warning: fact in <path> does not begin with topic '<title>': <fact>"`). Do not skip or modify the fact.
  - [ ] 3.7 Implement `ExtractWikiLinks(sentence string) []string` that finds all `[[...]]` occurrences and returns their inner texts in order of appearance.
  - [ ] 3.8 Write `internal/parser/note_test.go` covering: title extraction; single-tag line; multi-tag line; hierarchical tag; fact splitting on blank lines; wiki-link extraction; warning on fact missing TOPIC subject.

- [ ] 4.0 Deterministic Card Generator
  - [ ] 4.1 In `internal/generator/generator.go`, define `type Card struct { NoteID int64; SentenceHash string; ClozeKey string; Kind string; Front string; Back string; Source string }`. `Source` is always `"template"` in this phase.
  - [ ] 4.2 Implement `SentenceHash(sentence string) string` following §6.1 exactly: trim → replace `[[X]]` with `X` → lowercase → collapse whitespace runs to one space → strip leading and trailing `.` and `,` → return `hex(sha256(result))`.
  - [ ] 4.3 Implement `ClozeKey(linkText string) string`: lowercase the inner text of a wiki-link. For definition and property cards, `ClozeKey` is `""`.
  - [ ] 4.4 Implement `GenerateCards(note *parser.Note) []Card` following the table in §5.1:
    - For the sentence containing `is` or `are` (first match only): generate a `definition` card with front `"What is <TOPIC>?"` and back = full sentence.
    - For every sentence containing wiki-links: generate one `cloze` card per distinct link; front = sentence with that link's text replaced by `___` (leave other links intact); back = full sentence; `ClozeKey` = lowercased link inner text.
    - For every sentence with neither `is`/`are` nor wiki-links: generate one `property` card; front = sentence with everything after `<TOPIC> <first-token>` replaced by `___`; back = full sentence; `ClozeKey` = `""`.
    - If the `is`/`are` sentence also contains wiki-links, produce **both** the definition card **and** one cloze card per link (§5.1 note 1).
  - [ ] 4.5 Ensure `GenerateCards` is pure and deterministic: calling it twice with the same `*Note` must produce an identical `[]Card` slice in the same order every time.
  - [ ] 4.6 Add `notecards generate` CLI command in `internal/cli/`: load config → scan notes_dir → parse each note → generate cards → if `--json` print JSON array, else print human-readable table. No database access. This is the Phase 1 deliverable.
  - [ ] 4.7 Write `internal/generator/generator_test.go` covering:
    - One `is` sentence + one two-link sentence → 1 definition card + 2 cloze cards (acceptance criterion 1).
    - Regenerating the same note produces the identical card set (acceptance criterion 1).
    - Renaming a wiki-link (cosmetic) → same `SentenceHash` (acceptance criterion 2).
    - Changing trailing punctuation → same `SentenceHash` (acceptance criterion 2).
    - Rewording a fact → different `SentenceHash` (acceptance criterion 3).
    - `is` sentence with two wiki-links → definition card + 2 cloze cards.

- [ ] 5.0 SQLite Data Layer & Schema
  - [ ] 5.1 In `internal/store/db.go`, implement `Open(path string) (*sql.DB, error)`: use `modernc.org/sqlite` driver; create parent directories with `os.MkdirAll`; open with `?_foreign_keys=on`; then call `applyMigrations(db)`.
  - [ ] 5.2 Implement `applyMigrations(db *sql.DB)`: execute the full schema from §7 of the PRD as a single `CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS` transaction. Include all six tables: `notes`, `note_tags`, `cards`, `reviews`, `review_log`, `resolution_log`.
  - [ ] 5.3 Implement `NoteRepository` in `internal/store/notes.go` with methods: `Upsert(note)` (insert or update by `path`), `GetByPath(path) (*Note, error)`, `ListAll() ([]*Note, error)`.
  - [ ] 5.4 Implement `NoteTagRepository` in `internal/store/tags.go` with method `Replace(noteID int64, tags []string)`: delete all existing tags for that note, then insert the new list. Wrap in a transaction.
  - [ ] 5.5 Implement `CardRepository` in `internal/store/cards.go` with methods: `Upsert(card)` (insert or ignore on the UNIQUE constraint), `GetByNoteID(noteID) ([]*Card, error)`, `SetStatus(cardID int64, status string)`, `ListOrphaned() ([]*Card, error)`, `ListActiveByNoteID(noteID int64) ([]*Card, error)`.
  - [ ] 5.6 Implement `ReviewRepository` in `internal/store/reviews.go` with methods: `Create(cardID int64)` (insert a fresh `state=new` row), `GetByCardID(cardID) (*Review, error)`, `Update(review)` (replace all scheduling fields after a grade).
  - [ ] 5.7 Implement `ReviewLogRepository` in `internal/store/reviews.go` with method: `Insert(cardID int64, grade int)`.
  - [ ] 5.8 Implement `ResolutionLogRepository` in `internal/store/resolution.go` with methods: `Insert(batchID, action string, orphanCardID int64, targetCardID *int64)`, `GetLastBatch() ([]ResolutionEntry, error)`, `DeleteBatch(batchID string)`.
  - [ ] 5.9 Write `internal/store/store_test.go`: open an in-memory SQLite DB for each test; verify UNIQUE constraint on `cards(note_id, sentence_hash, cloze_key)`; verify `Replace` on `note_tags` clears and repopulates; verify `ResolutionLog` insert and retrieval.

- [ ] 6.0 FSRS Integration + Plain-Stdout `review` Command
  - [ ] 6.1 In `internal/scheduler/fsrs.go`, implement `Grade(current Review, grade int, now time.Time) Review`: convert the `reviews` row to a `go-fsrs` card struct, call `fsrs.Repeat`, pick the scheduled result for the given grade, and map it back to a `Review` row.
  - [ ] 6.2 Implement `DueCards(db, now, tagFilter, matchMode, excludeTags, noteTitle, limit) ([]*Card, error)` in `internal/store/cards.go`: query `cards JOIN reviews` where `reviews.due <= now AND cards.status = 'active'`; apply tag JOIN when tags are provided; apply `--match all` (intersection) with a `GROUP BY / HAVING COUNT` approach; apply `--exclude-tag` as a NOT EXISTS subquery.
  - [ ] 6.3 Implement the `review` command in `internal/cli/review.go`: load due cards → for each card: print front, wait for any keypress, print back, prompt "Grade (1=again 2=hard 3=good 4=easy): ", read single digit, call `Grade`, persist updated `Review` + `ReviewLog` row, repeat until no cards or limit reached.
  - [ ] 6.4 Apply config defaults (`review.tags`, `review.match`, `review.exclude`) when no `--tag` / `--match` / `--exclude-tag` flags were given on the CLI; if CLI flags are provided, discard config defaults entirely (no merge).
  - [ ] 6.5 Write `internal/scheduler/fsrs_test.go`: grade `again` (1) on a review-state card → `lapses` incremented and new `due` < old `due`; grade `good` (3) on a new-state card → `state` advances and `due` is in the future.
  - [ ] 6.6 Write an integration test for the full plain-stdout review loop using an in-memory DB.

- [ ] 7.0 `sync` Command — Incremental Reindex & Orphan Detection
  - [ ] 7.1 In `internal/sync/sync.go`, implement `ScanNotes(notesDir string) ([]NoteFile, error)`: walk the directory, collect all `.md` files, compute SHA-256 of each file's content, return `[]NoteFile{Path, ContentHash}`.
  - [ ] 7.2 Implement `Sync(db, notesDir string) (SyncResult, error)`: compare `NoteFile.ContentHash` against `notes.content_hash` in the DB; skip unchanged files; process only new or changed files.
  - [ ] 7.3 For each changed/new file: call `parser.ParseNote` → call `NoteRepository.Upsert` → call `NoteTagRepository.Replace` → call `generator.GenerateCards` → for each generated card, call `CardRepository.Upsert`; if newly inserted (row count > 0), call `ReviewRepository.Create`.
  - [ ] 7.4 After processing all live files, detect orphans: for each note in the DB, fetch its active cards; any card whose `(note_id, sentence_hash, cloze_key)` did NOT appear in the regenerated set → call `CardRepository.SetStatus(id, "orphaned")`.
  - [ ] 7.5 Handle deleted notes: for every note path in the DB that is NOT in `ScanNotes` results, orphan all its active cards (same `SetStatus` call, iterating `GetByNoteID`).
  - [ ] 7.6 Return a `SyncResult{Unchanged, New, Orphaned int}` and print the summary line: `"X unchanged, Y new, Z orphaned — run 'mneme resolve'"`. If `Z == 0`, omit the resolve hint.
  - [ ] 7.7 **Critical check:** confirm that `Sync` never inserts into `resolution_log` and never calls any remap/discard operation. Add an assertion or comment making this explicit.
  - [ ] 7.8 Implement `--watch` flag: after the initial sync, use `fsnotify` to watch `notesDir` and re-run `Sync` on any `.md` file change event. Print the summary after each re-sync.
  - [ ] 7.9 Write `internal/sync/sync_test.go`:
    - After `sync`, `resolution_log` is empty and no `reviews.card_id` changed owner (acceptance criterion 4).
    - Cosmetic wiki-link rename → `SyncResult.Orphaned == 0`; card history intact (acceptance criterion 2).
    - Rewording a fact → `SyncResult.Orphaned == 1`, `SyncResult.New == 1`; orphaned card has no reviews row reuse (acceptance criterion 3).
    - Deleting a note file → its cards become `orphaned`, not deleted (acceptance criterion 9).

- [ ] 8.0 Manual Resolver — `orphans`, `resolve`, `resolve --undo`
  - [ ] 8.1 Implement `notecards orphans` in `internal/cli/orphans.go`: call `CardRepository.ListOrphaned`, join with `reviews` to fetch `reps`, `lapses`, `due`; print each orphan as one line: `[id] <sentence> | reps=N lapses=N due=<date>`.
  - [ ] 8.2 Implement `notecards resolve` in `internal/cli/resolve.go`: generate a `batchID` (e.g., `fmt.Sprintf("%d", time.Now().UnixNano())`); fetch pending orphans; iterate one at a time.
  - [ ] 8.3 For each orphan, display: the orphan sentence and its review history; then a numbered list of candidate active cards defaulting to the same note (show note title, sentence, kind).
  - [ ] 8.4 Implement candidate widening: after the default same-note list, offer options `[w]` to widen to recently-created cards across all notes, and `[/keyword]` to filter candidates by a keyword in the note title or sentence.
  - [ ] 8.5 Implement action prompt with these exact keys — **no pre-selection, Enter does nothing**:
    - `r <number>` — remap: user types `r` then the candidate number, then presses a confirmation key that is **not** Enter (e.g., `y`).
    - `d` followed by confirmation key `y` — discard.
    - `s` — skip (no confirmation needed; skip is non-destructive).
    - Any other key, including bare Enter → reprint the prompt.
  - [ ] 8.6 Implement the remap transaction in `internal/store/resolution.go` or a dedicated `Remap(db, orphanID, targetID int64, batchID string)` function following §7.1 exactly:
    1. Delete target's blank `reviews` row (if `reps == 0`).
    2. `UPDATE reviews SET card_id = targetID WHERE card_id = orphanID`.
    3. `UPDATE review_log SET card_id = targetID WHERE card_id = orphanID`.
    4. `UPDATE cards SET status = 'superseded' WHERE id = orphanID`.
    5. `INSERT INTO resolution_log (batch_id, action, orphan_card_id, target_card_id, created_at)`.
    All five steps in a single transaction; roll back on any error.
  - [ ] 8.7 Implement discard: `UPDATE cards SET status = 'superseded' WHERE id = orphanID`; insert `resolution_log` row with `action = 'discard'`, `target_card_id = NULL`.
  - [ ] 8.8 Implement `notecards resolve --undo`: call `ResolutionLogRepository.GetLastBatch`; iterate entries in reverse order; for each `remap` entry, reverse the transaction (repoint `reviews`+`review_log` back to orphan, set orphan → `active`, set target → `active`); for each `discard`, set orphan → `orphaned`; then call `DeleteBatch(batchID)`.
  - [ ] 8.9 Write `internal/cli/resolve_test.go` (or a corresponding integration test):
    - Remap transfers `reps`, `lapses`, `due` from orphan to target; orphan is `superseded` (acceptance criterion 5).
    - `resolve --undo` restores the exact prior state (acceptance criterion 5).
    - Pressing Enter alone does not change any DB row (acceptance criterion 6).
    - Cross-note remap: orphan from note A can select a card in note B as target (acceptance criterion 7).

- [ ] 9.0 `stats` Command & `--json` Global Flag
  - [ ] 9.1 Implement `notecards stats` in `internal/cli/stats.go`: run four queries in a single function and print results:
    - Total active cards: `SELECT COUNT(*) FROM cards WHERE status = 'active'`.
    - Due today: `SELECT COUNT(*) FROM cards JOIN reviews ON … WHERE reviews.due <= <end-of-today> AND cards.status = 'active'`.
    - Due in 7 days (forecast, day-by-day): group by `DATE(reviews.due)` for the next 7 days.
    - Orphan count: `SELECT COUNT(*) FROM cards WHERE status = 'orphaned'`.
  - [ ] 9.2 Add a global `--json` flag to the root command. When set, each command's output function checks the flag and emits a JSON object instead of human text. Define a `Output` interface or helper that all commands use, so the flag propagates cleanly. Example JSON for `stats`: `{"active":42,"due_today":7,"forecast":[{"date":"2026-09-17","due":3},…],"orphaned":2}`.
  - [ ] 9.3 Add `--json` to `sync`, `orphans`, `generate`, and `stats`; the `review` command output is interactive and does not need JSON output.
  - [ ] 9.4 Write unit tests for the stats queries against an in-memory DB, and a snapshot test for the JSON output shape of `stats`.

- [ ] 10.0 Bubbletea TUI — Review Screen, Resolver Screen, Stats
  - [ ] 10.1 Add `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss` to `go.mod` (they may already be present from task 1.1; verify with `go mod tidy`).
  - [ ] 10.2 Implement the review TUI model in `internal/tui/review.go` using the Bubbletea `Model` / `Update` / `View` pattern:
    - State machine: `showFront` → keypress → `showBack` → grade keypress (1–4) → FSRS + persist → next card or done screen.
    - Display progress: `Card 3 of 12`.
    - Use lipgloss for styling: bold front, dim back label, colored grade options.
  - [ ] 10.3 Implement the resolver TUI model in `internal/tui/resolve.go`:
    - Left panel: orphan sentence + review history.
    - Right panel: numbered candidate list with note title and sentence.
    - Bottom bar: action keys `r<n>y` / `dy` / `s` with a clear label; Enter does nothing.
    - When a candidate is focused, show a diff between the orphan sentence and the candidate sentence (highlight changed words using a simple word-diff).
  - [ ] 10.4 Implement the stats TUI screen in `internal/tui/stats.go`: show active, due-today, orphan count, and a simple 7-day forecast table or bar (using lipgloss box characters).
  - [ ] 10.5 Wire TUI as the default for `review` and `resolve`: when a terminal is detected (`term.IsTerminal(os.Stdout.Fd())`), launch the TUI model; when non-interactive or `--no-tui` is passed, fall back to the plain-stdout path from tasks 6 and 8.
  - [ ] 10.6 Manual smoke-test checklist (no automated test for TUI rendering):
    - [ ] Run `mneme sync` against a sample notes directory; confirm summary counts are correct.
    - [ ] Run `mneme review`; confirm front/back display, grade keys work, card advances.
    - [ ] Manually edit a note to change a fact; run `mneme sync`; confirm orphan appears.
    - [ ] Run `mneme resolve`; confirm Enter does nothing; remap an orphan; confirm `mneme orphans` shows one fewer.
    - [ ] Run `mneme resolve --undo`; confirm the remap is reversed.
    - [ ] Run `mneme stats`; confirm counts match expectations.
    - [ ] Run `mneme stats --json`; confirm valid JSON output.
