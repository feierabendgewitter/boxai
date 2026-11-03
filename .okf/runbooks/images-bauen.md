---
type: Runbook
title: Images bauen
status: active
tags: [images]
---
# Voraussetzungen

Docker läuft, `sbx` ist angemeldet (`boxai doctor --docker`).

# Bauen

`make images` baut alle Varianten und lädt sie mit `sbx template load`. Einzelne Variante: `images/build.sh claude kotlin`. Die Skript-Variablen stehen am Anfang von `images/build.sh`.

# Prüfen

`sbx template ls | grep boxai-` listet die geladenen Images.
