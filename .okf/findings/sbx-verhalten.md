---
type: Finding
title: Reales Verhalten von sbx
status: verified
tags: [sbx, phase-0]
---
# Reales Verhalten von sbx

Geprüft am 2026-11-03 auf macOS (arm64) mit **sbx v0.51.0**. Diese Seite hat Vorrang vor den Annahmen in `boxai-building-plan.md`. Mindestversion: siehe `MinSbxVersion` in `internal/app/doctor`.

## Abweichungen vom Plan

| Plan-Annahme | Reales Verhalten | Konsequenz |
| --- | --- | --- |
| Built-in-Agents wählen v2-Kits | `claude`, `copilot`, `shell` sind v3-Kits (`docker.io/sbx/<agent>`). `--kit` mit v2-Mixin scheitert: `cannot mix v3 kits with v1/v2 kits` | [Kits v3](../decisions/0004-kits-v3.md) |
| `files/home/`, `files/workspace/` | Gibt es in v3 nicht; Dateien stehen als `lifecycle@1`-`files` im Deskriptor | [kitfs](../architecture/adapters/kitfs.md) |
| `sbx kit validate --offline` prüft den Kit | Lehnt v3-Quell-Kits ab (`no kit builder configured`). Ohne `--offline` läuft es und braucht den Daemon | `KitWriter.Validate` ohne `--offline` |
| Kit setzt `environment.variables` | In v3 kein Mixin-Feld | `sbx create -e` |
| Secrets nach `create` (zu verifizieren) | Nur vor `create` landet der Platzhalter in der Env der Sandbox; danach bleibt die Variable leer | Secrets vor `SandboxRuntime.Create` |
| `boxai attach` über `sbx attach` | `sbx attach` ist nur `--cloud`; lokal gilt `sbx run --name` | `SandboxRuntime.Attach` |
| Registry-Domain bei `--image` Pflicht | Lokal geladene Images funktionieren ohne Domain mit `--pull never` | Referenz ohne Domain gilt als lokal |
| `sbx template inspect` | Nur `--cloud`; lokal `docker image inspect` | Label-Prüfung über Docker |

## Fallstricke

- **Skills-Store:** Standardmäßig hängt sbx einen read-only Skills-Store über `~/.<agent>/skills`. Kit-Dateien dort scheitern mit `Read-only file system` und die Fehlermeldung lautet nur `failed to apply kit`. boxai setzt `--skills off`.
- **Fehlerdetails:** Kit-Fehler stehen nur in `~/Library/Application Support/com.docker.sandboxes/sandboxes/sandboxd/daemon.log`.
- **Hänger:** sbx 0.46.0 hing hier bei jedem Aufruf in `_dyld_start`, ohne Ausgabe; das Update auf 0.51.0 behob es. Die Ursache wurde nicht geklärt. `execx` setzt deshalb Zeitlimits, und `doctor` meldet ein antwortloses Binary.
- **Teil-Maskierung:** `sbx secret ls --json` zeigt die ersten sechs Zeichen eines Werts. Der Adapter liest nur `placeholder` und gibt diese Ausgabe nie weiter.
- **`sbx rm` ist sbx-global benannt:** `--all`, `prune`, `reset` betreffen auch fremde Sandboxes; boxai entfernt immer namentlich.

## Beantwortete offene Punkte

| Frage | Antwort |
| --- | --- |
| Mehrere Zusatz-Workspaces, auch `.git` als Pfad? | Ja. `sbx create shell <ws> <repo-a>/.git <repo-b>/.git` funktioniert; alle Pfade liegen unter ihrem Host-Pfad |
| Git im Worktree in der Sandbox? | `status`, `add` und `commit` funktionieren in zwei Worktrees; die Commits sind auf dem Host sichtbar |
| Braucht `set-custom --sandbox` eine existierende Sandbox? | Nein |
| Wert über stdin? | Ja, `--value-stdin` |
| Sandbox-Secrets löschen? | `sbx rm` löscht sie mit. Einzeln: `sbx secret rm --placeholder <p> --sandbox <n> -f` |
| Bleiben Worktrees bei `sbx rm`? | Ja, Worktrees und Branches von boxai bleiben unberührt |
| Copilot-Pfade | `~/.copilot/agents/` (GitHub-Doku), `~/.copilot/skills/` (sbx mountet dort seinen Store) |
| Antigravity-Pfade | Workspace `.agents/skills/` (Docker-Doku); CLI heißt `agy`, Details in der [Registry](../domain/agent-registry.md) |
| Kits v2 oder v3? | v3, siehe [Entscheidung](../decisions/0004-kits-v3.md) |

## Tools der Templates

| Werkzeug | `sbx/claude` (Standard, Debian DHI) | `sandbox-templates:claude-code-docker`, `:copilot-docker` (Ubuntu 26.04) |
| --- | --- | --- |
| git, gh, jq, curl | ja | ja |
| Java | nein | OpenJDK 25 (Plan verlangt 21) |
| Node | nein | 22 (npm 9 beziehungsweise 10) |
| Maven, glab, aws | nein | nein |

Eigene Images über `sbx create -t <tag> --pull never` funktionieren zusammen mit `claude` und einem v3-Mixin.

## Nicht verifiziert

- Ein Maven-Zugriff gegen ein privates Repo mit echtem Token: kein solches Repo verfügbar.
- AWS-SigV4 mit Platzhaltern: laut Mechanik zum Scheitern verurteilt, weil die Signatur lokal entsteht; `secrets.mode: env` ist der Ausweg.
