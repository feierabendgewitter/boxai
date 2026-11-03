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

Unbekannte YAML-Felder sind Fehler. Relative Pfade gelten relativ zur Datei. `secrets` ist ein Pfad oder eine Abbildung mit `file` und `mode`.

Abweichung vom Plan: Das `.env`-Format hat einen eigenen Parser statt `godotenv`, weil dieses die `# @hosts`-Kommentare verwirft und `${NAME}` gegen die Datei statt gegen die Umgebung auflöst. Fehlermeldungen nennen nur Namen und Zeilennummern, nie Werte; deshalb ersetzt der Adapter auch YAML-Fehlertexte, die Eingaben zitieren können.
