# boxai – Einstieg für Agents

`boxai` bündelt mehrere Git-Repos als Worktrees zu einem Workspace und startet ihn in einer Docker Sandbox (`sbx`).

## Lies zuerst

1. [`.okf/index.md`](.okf/index.md): Projektwissen (Architektur, Entscheidungen, sbx-Erkenntnisse). Vor Arbeit an einem Adapter das passende Konzept unter `.okf/architecture/adapters/` lesen.
2. [`boxai-building-plan.md`](boxai-building-plan.md): die verbindliche Spezifikation. Weicht das reale `sbx`-Verhalten ab, gilt `.okf/findings/sbx-verhalten.md`.

## Abhängigkeitsregel

`adapter → app → domain`. `domain` und `app` importieren weder Adapter noch `os/exec`, `net` oder (in `domain`) `os`. I/O läuft über Ports in `internal/app/port`, verdrahtet nur in `cmd/boxai/main.go`. `depguard` und `internal/archtest` erzwingen das.

## Definition of Done

Eine Änderung ist fertig, wenn alle Punkte erfüllt sind:

- `make check` ist grün (Tests, Vet, Lint, OKF-Lint).
- `.okf/` ist nachgezogen: Entscheidung → `decisions/`, sbx-Erkenntnis → `findings/`, neuer Adapter → `architecture/adapters/`, Port-Änderung → `architecture/ports.md`, Phase → Eintrag in `log.md`.
- Jede Aussage steht genau einmal im Bundle; `README.md` und `AGENTS.md` verlinken nur.

## Tests

- Domäne: reine Unit-Tests ohne I/O.
- Use Cases: nur gegen die Fakes in `internal/app/port/porttest`.
- `gitcli`, `fsscan`: gegen echte Repos in `t.TempDir()`.
- `sbxcli`: mit aufzeichnendem Fake-Runner und Snapshots.
- `kitfs`, Parser: Golden Files unter `testdata/golden/` des Pakets; aktualisieren mit `go test ./internal/adapter/out/kitfs -update`.
- E2E: Build-Tag `e2e`, nur lokal mit `sbx` (`make e2e`).

## Arbeiten mit echtem sbx

- **Fremde Sandboxes sind tabu.** Auf diesem Rechner liegen Sandboxes des Nutzers. Lege Testobjekte nur mit dem Präfix `boxai-spike` oder `boxai-e2e` an und entferne ausschließlich diese, immer namentlich. `sbx rm --all`, `sbx reset`, `sbx prune` und `sbx secret rm --all` wirken global und bleiben ungenutzt.
- **Netzwerkregeln und Secrets scopen:** immer mit `--sandbox <name>`, damit die globale Policy des Nutzers unverändert bleibt.
- **Testwerte:** Echte Secrets aus dem Host bleiben außerhalb der Tests; nimm erfundene Werte.
- **Zeitlimit setzen:** `sbx` kann ohne Ausgabe hängen. Rufe es in Skripten mit Zeitlimit auf, etwa `perl -e 'alarm 60; exec @ARGV' sbx ls`.

## Secrets im Code

Secret-Werte erscheinen nie in Logs, State-Dateien, Kits oder Kommandozeilen. Übergib sie per `--value-stdin` über `execx` und markiere sie dort zur Redaction.
