---
type: Adapter
title: configfile
status: active
tags: [adapter]
---
# Verantwortung

`internal/adapter/out/configfile` implementiert `port.ConfigLoader`.

# Externe Abhängigkeit

`gopkg.in/yaml.v3`, Dateisystem.

# Fallstricke

Unbekannte YAML-Felder sind Fehler. Relative Pfade gelten relativ zur Datei.
