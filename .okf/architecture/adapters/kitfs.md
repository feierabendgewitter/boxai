---
type: Adapter
title: kitfs
status: active
tags: [adapter, kit, sbx]
---
# Verantwortung

`internal/adapter/out/kitfs` implementiert `port.KitWriter`. Es übersetzt die formatfreie `sandbox.KitSpec` in einen deklarativen [v3-Mixin](../../decisions/0004-kits-v3.md) unter `~/.boxai/kits/<name>/<stem>.yaml`.

# Externe Abhängigkeit

Schema `docker/sandbox-kit:3`; Validierung über `sbx kit validate` mit laufendem Daemon.

# Fallstricke

- Die erste Zeile muss `# syntax=docker/sandbox-kit:3` sein.
- Unbekannte Felder sind Fehler; der Adapter erzeugt daher nur `schemaVersion`, `kind`, `description` und `capabilities`.
- Ein `lifecycle@1`-Eintrag ohne Hooks und Dateien ist ungültig; ohne Dateien entfällt der Kit-Deskriptor.
- `${NPM_TOKEN}` in Dateiinhalten bleibt erhalten; nur `${{ kit.env.NAME }}` wird von sbx ersetzt.
- Der Kit-Ordner enthält nie Secret-Werte.
