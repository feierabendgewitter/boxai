---
type: Adapter
title: execx
status: active
tags: [adapter, subprozess]
---
# Verantwortung

`internal/adapter/out/execx` ist der gemeinsame Subprozess-Runner aller Driven Adapter. Er implementiert keinen Port, sondern das Interface `execx.Runner`.

# Regeln

- **Dry-Run:** Führt nur `Cmd.ReadOnly` aus; alles andere landet als Plan in `PlanOut`.
- **Redaction:** `Exec.Redact` und `Cmd.Secrets` entfernen Werte aus Ausgaben, Logs und Fehlern. `Format` zeigt Stdin und Env-Werte nie.
- **Zeitlimit:** Nicht-interaktive Kommandos enden nach `DefaultTimeout`, weil [sbx hängen kann](../../findings/sbx-verhalten.md).
- **Exit-Code:** `ExitError` trägt ihn, damit `boxai attach` ihn übernehmen kann.

# Tests

`execxtest.Recorder` zeichnet Kommandos für Snapshot-Tests der Adapter auf.
