---
name: atomic-note
description: Write an atomic note about a concept, in the format mneme turns into flashcards. Use when the user asks to write, create, or update an atomic note (or a mneme note) about a topic. The topic is required as input every time; the destination root is configured in this skill.
argument-hint: <topic>
---

# Atomic note writer

Write one atomic note about the topic given as input, in the format mneme
parses into flashcards.

## Configuration

Edit this value to change where notes are written. Keep it equal to
`notes_dir` in `~/.config/mneme/config.yaml`, otherwise mneme won't see the
notes.

```
NOTES_ROOT: ~/Documents/RedHatNotes/Resources
```

## Input

- **Topic** (required, every time): `$ARGUMENTS`

If no topic was given, ask for one and stop. Never guess a topic from the
conversation. Never ask for the destination; it comes from `NOTES_ROOT`.

## Steps

1. **Resolve the title.** Use the topic's usual name, preferring the acronym
   when that is how it is commonly referred to (`CNI`, not `Container Network
   Interface`). Every sentence of the note starts with this exact string. If
   the topic is ambiguous (e.g. "Operator": Kubernetes or math?), ask which
   meaning before writing.
2. **Locate the file:** `<NOTES_ROOT>/<TITLE>.md`, expanding `~` and env vars.
   Replace `/` in the title with `-` for the file name only.
3. **If the file already exists**, read it and **add** only facts that are
   missing. Don't rewrite or reorder existing facts: mneme ties study history
   to each sentence, and rewording one orphans its cards. Report what you
   added.
4. **Gather the facts.** Use what you know; if a web search tool is available
   and the topic is recent or niche, verify with it. Keep only permanent,
   checkable knowledge: no opinions, versions likely to change soon, or
   tutorial steps. Aim for 4–10 facts; fewer good facts beat many weak ones.
5. **Write the note** following the format below, then **check it** against the
   checklist. If the `mneme` binary is available, run `mneme generate --unmarked`
   and read the card for each fact; fix any card that reads badly.
6. **Reply** with the file path and the list of cards the note produces
   (front → answer), so the user can review them without opening the file.

## Note format

```markdown
# <TITLE>
#research
#study
#area/<domain>

<TITLE> is :: <one-line definition>.

<TITLE> <verb> **<answer>** <rest of the sentence>.

<TITLE> <verb> <cue> :: <answer>.

<Explicit question about TITLE>? :: <answer>
```

- **Header:** `# <TITLE>` on the first line, then tag lines directly below it,
  with no blank line in between. Always include `#research` and `#study`; add
  one or more `#area/<domain>` tags (lowercase, hyphenated, e.g.
  `#area/kubernetes`). Then a blank line.
- **Facts:** one fact per paragraph, one sentence per fact, separated by blank
  lines. Never use bullet lists, headings, or multi-sentence paragraphs in the
  body.
- **Topic first:** every fact starts with `<TITLE>` as the subject. The only
  exception is an explicit question (`… ? :: …`).
- **The first fact is the definition** and uses the cue `<TITLE> is ::` (or
  `<TITLE> are ::`) exactly. mneme turns it into "What is <TITLE>?".
- **Wiki-links** `[[Concept]]` mark related concepts that have (or deserve)
  their own note. Use the target's title, acronym form preferred.
- **Code, commands, paths, URLs, and values** go in backticks:
  `` `kubectl apply -f` ``, `` `/etc/kubernetes` ``, `` `6443` ``.

## Marking the answer

mneme never guesses. Each fact makes cards only from the markers you put in it:

| Marker | Card |
|---|---|
| `**span**` | cloze card with that span blanked (one card per bold span) |
| `cue :: answer` | cloze card `cue ___` |
| `Question? :: answer` | question card |
| `<TITLE> is :: definition` | "What is <TITLE>?" |
| no marker | no card |

Choose markers so that **the visible words narrow the answer down to one
thing**:

- Good: `CNI is used by **[[Kubernetes]]** to configure Pod networking.` Only
  one answer fits.
- Bad: `CNI is :: used by Kubernetes to configure Pod networking.` for a
  non-definition fact: dozens of answers would fit `CNI is ___`.
- Keep answers short: a name, a term, a number, a short phrase. If the answer
  needs a whole clause, split the fact in two.
- Use the real verb, not wrappers like `is a way to`, `is used to`, `allows you
  to`: write `X builds **Y**`, not `X is a way to build Y`.
- Prefer `**bold**` for answers in the middle of a sentence and `::` for
  answers at the end. Use at most one `::` per fact.
- Bold a wiki-link (`**[[etcd]]**`) to make it the answer. A plain wiki-link
  is just a link.
- Mark the important part, not every noun: one or two blanks per fact.
- Markers inside backticks are ignored, so `` `std::vector` `` is safe, but a
  `::` or `**` meant as a marker must sit outside backticks.
- A fact worth keeping but not worth studying may stay unmarked.

## Checklist

- [ ] File is `<NOTES_ROOT>/<TITLE>.md`; existing facts were not changed.
- [ ] Title line, tag lines (`#research`, `#study`, `#area/...`), blank line.
- [ ] Each paragraph is one sentence starting with `<TITLE>` (or an explicit
      question).
- [ ] First fact is `<TITLE> is :: …`.
- [ ] Every studied fact has a marker; no fact has more than one `::`.
- [ ] Each cue leaves exactly one sensible answer.
- [ ] Code, paths, commands and values are in backticks.

## Example

Input: `etcd`

```markdown
# etcd
#research
#study
#area/kubernetes

etcd is :: a distributed, consistent key-value store.

etcd reaches agreement between members using the **[[Raft]]** consensus algorithm.

etcd stores the cluster state of :: [[Kubernetes]].

etcd needs a majority of members, called a **quorum**, to accept writes.

etcd serves client requests on port :: `2379`.

Which port does etcd use for peer communication? :: `2380`
```
