# Working on downguy

## Before you push

```
make check
```

That is format, vet, the project lint, build, the whole suite, the coverage
floor and the command reference. It is the same set the pipeline runs, so a
green `make check` is a green pipeline apart from the container image and the
race detector, which both need something this machine may not have.

## Where things go

Tests live in `tests/` as an external `package tests`. Nothing under
`internal/` carries a `_test.go` file, and the lint refuses one. The reason is
that everything the tests reach is then part of a package's real interface: a
test that pokes at an unexported field is a test that stops you changing it.

Anything that changes plant goes through the command line. The http service is
read only and the lint will not let a package under `internal/` reach for the
process's own streams, because a command that writes to `os.Stdout` directly
cannot be tested without a subprocess.

## The shapes that have to agree

Four documents pass between commands and they have broken against each other
more than once:

- `makeready -json` writes a plan **and the attributed findings behind it**
- `estimate` and `packet` read that plan
- `allocate` reads the findings off the same document
- `bill` reads the plant, not the plan

`tests/endtoend_test.go` walks all of it. If you change one of those shapes,
that is the test that will tell you what else moved.

## Numbers

Heights are integer tenths of a foot. Money is integer cents. Rates are basis
points. Nothing in a record holds a float, and the formatting lives in
`internal/unit` rather than in whichever package is printing.

There is one place that prints a number without a unit on it, and it is
`unit.Whole`, `unit.Decimal` and `unit.Plain`. Four packages grew their own
copies once; they agreed until one of them changed.

## Rules and findings

Every rule that can fire has a stable id in `internal/nesc/rule.go`, and every
finding carries the id it came from. The lint checks that a rule cited anywhere
in the engine is in the catalogue, because a finding citing something nobody
can look up is worse than no finding.

A rule does not decide blame. It sees one pole in one state and cannot tell a
condition it caused from one it inherited, so it emits `unattributed` and
`clearance.Attribute` decides by comparing the pole with and without the
applicant's facilities.

## Golden files

The printed reports are compared against `tests/testdata/*.golden`. After a
change that is meant:

```
make golden
```

Read the diff before you commit it. That is the only thing standing between a
report and a column quietly moving.

## The advisory gate

`make advisories` runs the vulnerability scanner and judges it against
`security-advisories.json`. Anything outside the standard library fails the
build. Standard library findings are accepted only against a written entry with
a review date, and **the build goes red on that date by design**. If you are
reading this because the pipeline just went red on it, the answer is to read
the advisories again, not to move the date.

## Commit style

One change per commit, with a message that says what the change does in the
present tense and lower case. The history is the explanation; the code is what
it explains.
