# AGENTS.md

Repository-specific conventions for anyone — human or agent — changing this
code. The code style itself lives in the parent host's AGENTS.md and applies
here unchanged: no comments, no abbreviations, small functions, guard clauses.

## Working on this repository

Nothing lands on `main` by direct push. Branch, open a pull request, let the
check run, merge.

### Branch names

`<type>/<subject-in-kebab-case>` — the type is the same word the commit will
carry, and the subject says what changes, not what you did to it.

```
feat/acp-context-injection
fix/approval-resume-after-restart
docs/tui-screenshots
refactor/turn-runner-split
test/completion-gate-evidence
chore/gofmt
ci/skip-docs-only-runs
```

Branch off `main`, rebase rather than merge when `main` moves under you, and
delete the branch once the pull request is merged.

### Commit messages

```
<type>: <what changes, imperative, lowercase, no trailing period>

<why it changes: the problem the reader would otherwise have to reconstruct.
Wrap at 72 columns. Say what you deliberately did not do.>
```

`type` is one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`. A
scope is allowed when it disambiguates (`fix(acp): …`) and omitted when it does
not.

The subject line is a claim about the code, not a description of the work:
`fix: stop reading "the agent stopped talking" as "the task is done"` rather
than `fix: fixed the status bug`. The body exists to answer *why*; a commit
whose body only repeats the subject should not have one.

### Pull requests

One reviewable change per pull request. The description says what the reader
should look at and what evidence exists that it works — the test that fails
without the change, the scenario that was run, the screenshot. A pull request
that touches unrelated files should be split.

Branch names, commit messages, pull request titles and pull request
descriptions are written in English. Discussion in review can be in whatever
language the reviewers share; the repository's permanent record is English.
