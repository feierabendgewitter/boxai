---
type: Architecture
title: Hexagonale Architektur
status: active
tags: [architektur]
---
# Schichten

Abhängigkeiten zeigen nur nach innen: `adapter → app → domain`. Verdrahtet wird ausschließlich in `cmd/boxai/main.go`.

| Schicht | Pfad | Darf importieren |
| --- | --- | --- |
| Domäne | `internal/domain/*` | Standardbibliothek ohne `os`, `os/exec`, `net` |
| Use Cases | `internal/app/*` | Domäne, Ports; Standardbibliothek ohne I/O |
| Ports | `internal/app/port` | Domäne |
| Driving Adapter | `internal/adapter/in/cli` | Ports, Domäne, Cobra, huh |
| Driven Adapter | `internal/adapter/out/*` | Ports, Domäne, Drittbibliotheken |

# Durchsetzung

- `depguard` in `.golangci.yml` prüft direkte Imports.
- `internal/archtest` prüft mit `go list`: transitiv weder Adapter noch `os/exec` im Kern, direkt kein `os` und kein `net`. Transitiv steckt `os` bereits in `fmt`, deshalb zählt dafür der direkte Import.

# Dry-Run

Der Composition Root baut Adapter erst, wenn die Flags bekannt sind (`cli.Factory`). Im Dry-Run führt [execx](adapters/execx.md) nur Kommandos mit `ReadOnly` aus und gibt die übrigen als Plan aus. Use Cases kennen den Modus nicht.
