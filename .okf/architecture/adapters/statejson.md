---
type: Adapter
title: statejson
status: active
tags: [adapter]
---
# Verantwortung

`internal/adapter/out/statejson` implementiert `port.StateStore`.

# Externe Abhängigkeit

Dateisystem, `~/.boxai/state/<name>.json`.

# Fallstricke

Dateien entstehen atomar mit Modus 0600 und enthalten nie Secret-Werte.
