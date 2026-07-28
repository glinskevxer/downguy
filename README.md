# downguy

Joint use pole attachment engineering and settlement, for the pole owner side.

An electric utility owns the distribution poles. Telephone, cable and fibre
companies rent space on them. Every new attachment has to be applied for,
surveyed, engineered and, when the pole cannot take it as it stands, made ready.
Somebody then has to decide who pays for that make ready and bill the annual
rent for every attachment that ends up on the pole. That whole loop is what
this runs.

## The chain of decisions

The loading district decides the ice and wind case. The ice and wind case
decides the conductor tension, which decides the sag, which decides where the
wire actually sits at mid span, which decides whether the ground clearance rule
is met. The same district decides the transverse load on the pole, which with
the species fibre stress and the class circumference decides the groundline
utilisation. Utilisation decides whether the remedy is a transfer or a pole
replacement, that decides the cost, and the cost is what gets allocated.

Nothing in that chain can be worked out on its own. That is why the rules are
an engine rather than a checklist.

## Getting started

```
make build
./bin/downguy init -db plant.db
./bin/downguy import -db plant.db poles poles.csv
./bin/downguy import -db plant.db attachments attachments.csv
./bin/downguy import -db plant.db spans spans.csv
./bin/downguy import -db plant.db surveys surveys.csv
./bin/downguy clear -db plant.db NW-441829
```

`scripts/smoke.sh` runs that whole sequence against a throwaway database and is
the fastest way to see the shape of the thing.

## Exit codes

The command line is meant to be driven from a pipeline, so the codes carry
meaning:

| code | meaning |
| --- | --- |
| 0 | ran, found nothing to report |
| 1 | ran, found something: violations, make ready, an overloaded pole |
| 2 | the command line itself was wrong |
| 3 | something failed that the caller cannot fix |

A clearance check that finds a violation exits 1. That is a result, not a
failure, and a pipeline that treats every non zero exit as a crash will get
this wrong.

## Commands

| command | what it does |
| --- | --- |
| `allocate` | decide who pays for the make ready and raise the invoices |
| `application` | open an application, move it on, and show where it has got to |
| `audit` | find unauthorized attachments and work out what they owe |
| `bill` | run the annual rental billing for a rate year |
| `chase` | list the clocks about to run out and who to chase about them |
| `clear` | run the clearance rules against a surveyed pole |
| `conductors` | list the catalogued conductors |
| `compliance` | say where the time went on an application and whose delay it was |
| `config` | print the settings a run would use |
| `costbook` | print the unit cost book as it stood on a day |
| `demo` | load a small worked example into an empty database |
| `estimate` | price a make ready plan against the unit cost book |
| `export` | write plant back out as csv |
| `history` | read the change log for a record, a person or a date |
| `import` | load a plant or survey export into the database |
| `init` | create or migrate the database |
| `inspect` | list the poles a groundline inspection programme should visit next |
| `load` | run the groundline loading check on a pole |
| `maintain` | count what is on file, check it hangs together, and compact it |
| `makeready` | plan the work a pole needs before an attachment can go up |
| `packet` | split a make ready plan into the sheets each crew is handed |
| `pole` | show a pole and everything on it |
| `policies` | list the cost allocation policies |
| `rate` | work out the annual attachment rate for a year |
| `rules` | list the code rules this system checks |
| `sag` | work out a sag and tension run for a conductor |
| `serve` | run the read side http service |
| `sweep` | run the clearance rules across every surveyed pole |
| `version` | print the build information |

`downguy help <command>` prints the options each one takes. The table above is
checked against the binary by `scripts/command-reference.sh`, so a command
added without a line here fails the build.

## One application end to end

```
downguy makeready -db plant.db -json -applicant 'MERIDIAN FIBER' NW-441829 > plan.json
downguy estimate  -on 2025-06-01 -quote < plan.json
downguy packet    < plan.json
downguy allocate  -on 2025-06-01 -findings plan.json \
    -owner 'SUMMIT VALLEY ELECTRIC=OWN-1' \
    -applicant 'MERIDIAN FIBER=ATT-9' < plan.json
```

`plan.json` carries both the work and the attributed findings behind it, which
is what lets the allocation say who caused each item rather than guessing.
`downguy demo` seeds a database this runs against.

## Layout

| package | what it owns |
| --- | --- |
| `internal/unit` | feet and inches, decimal feet, money in cents, rates in basis points |
| `internal/calendar` | effective dated records, rate years, proration |
| `internal/plant` | poles, spans, framing, attachments, anchors, the class tables |
| `internal/nesc` | loading districts, grades, clearance tables, rule ids |
| `internal/sagten` | conductor properties, the sag tension solver, ruling span |
| `internal/loading` | wind and ice loads, groundline moment, pole capacity |
| `internal/survey` | field capture, tolerances, reconciliation against the record |
| `internal/clearance` | the rule engine and the attribution behind every finding |
| `internal/makeready` | remedy selection and the work order it produces |
| `internal/costing` | the unit cost book and the estimate built from it |
| `internal/allocate` | who pays, under the pre existing violation rules |
| `internal/rental` | cable and telecom rate formulas, annual invoices |
| `internal/permit` | application lifecycle, the state machine and its clocks |
| `internal/store` | sqlite schema, migrations, repositories |
| `internal/ingest` | csv importers and exporters |
| `internal/report` | text and json renderings |
| `internal/apiserver` | the read side http service |
| `internal/cliapp` | argv dispatch, exit codes, subcommands |

## Conventions

Tests live in `tests/` at the repository root as an external `package tests`,
so everything they reach is part of a package's actual interface.

Heights are integer tenths of a foot and money is integer cents. Nothing in a
record holds a float.

Every rule that can fire has a stable id and every finding carries the id it
came from.

## The service

`downguy serve` runs a read only http service. It answers about plant, the
published rates, the code rules, applications, and a live clearance check on
one pole:

```
GET /healthz                 liveness
GET /readyz                  readiness, including the schema version
GET /poles?district=NW       the pole index
GET /poles/NW-441829         one pole with everything on it
GET /spans/SP-1              one span
GET /attachers/RIDGEPORT%20CABLEVISION
GET /applications?state=submitted
GET /applications/APP-1042
GET /rates/2025?attachers=5
GET /rules?section=235
GET /findings/NW-441829      the clearance rules run now, against the latest survey
```

There is no write side. Anything that changes a record goes through the command
line, where it is a file somebody can look at and an exit code a pipeline can
branch on.

A pole nobody has surveyed answers `/findings/` with a 404 rather than an empty
list. The difference matters: no findings and nobody has been out are not the
same answer.

## Development

```
make check      # format, vet, build, test, coverage floor
make race       # the race detector, where there is a C compiler
make shuffle    # the suite in a random order
make smoke      # one whole application through the built binary
make advisories # the vulnerability scan, judged against the acceptances
```

`security-advisories.json` records why the standard library findings against
the pinned toolchain are accepted, and the date the build goes red again if
nobody has looked at them.
