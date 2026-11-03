# boxai

`boxai` bündelt mehrere Git-Repos als Worktrees zu einem Workspace und startet ihn in einer [Docker Sandbox](https://docs.docker.com/ai/sandboxes/) mit Claude Code, GitHub Copilot CLI oder (experimentell) Google Antigravity. Ein Agent arbeitet so repo-übergreifend und bleibt vom Host isoliert.

## Voraussetzungen

- macOS oder Linux, jeweils `git`
- `sbx` ab v0.51: `brew install docker/tap/sbx`, danach `sbx login`
- Docker nur für `make images` und `--dockerfile`

## Installation

Aus einem [Release](https://github.com/feierabendgewitter/boxai/releases) oder mit Go:

```sh
go install github.com/feierabendgewitter/boxai/cmd/boxai@latest
boxai doctor
```

## Erster Start

```sh
cd ~/projekte          # Ordner, der deine Repos enthält
boxai up               # fragt Name, Repos und Agent ab
```

Ohne eigene Images startet der Agent mit seinem Standard-Image; `boxai` weist darauf hin, welche Tools dort fehlen. Die Images mit Profilen baut `make images` aus diesem Repository ([Details](.okf/runbooks/images-bauen.md)).

Nicht-interaktiv:

```sh
boxai up --name shop --repo backend --repo frontend --agent claude --profile kotlin-spring,react
boxai up --config boxai.yaml      # Vorlage: boxai init, Beispiel: examples/
boxai up --dry-run ...            # zeigt alle git- und sbx-Kommandos
```

## Kommandos

| Kommando | Zweck |
| --- | --- |
| `boxai up` | Worktrees und Sandbox anlegen, Agent starten; ein erneuter Lauf setzt fort |
| `boxai scan` | Repos unter einem Ordner auflisten |
| `boxai list` | Workspaces und Sandbox-Status |
| `boxai attach <name>` | Agent erneut starten |
| `boxai stop <name>` | Sandbox anhalten |
| `boxai rm <name>` | Sandbox, Worktrees, Kit und State entfernen; Branches bleiben ohne `--delete-branches` |
| `boxai doctor` | Voraussetzungen prüfen |
| `boxai init` | `boxai.yaml` und `secrets.example.yaml` schreiben |

## Wichtig zu wissen

Der Agent kann über den Mount von `<repo>/.git` Objekte und Branches deiner Haupt-Repos verändern ([warum](.okf/decisions/0005-worktree-mounts.md)). Secret-Werte bleiben im sbx-Store; die Sandbox sieht nur Platzhalter ([Details und Ausnahmen](.okf/domain/secrets.md)).

## Dokumentation

Das Projektwissen liegt als OKF-Bundle in [`.okf/`](.okf/index.md); Einstieg für Agents ist [`AGENTS.md`](AGENTS.md).
