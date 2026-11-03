---
type: Decision
title: Profile und Registry als Go-Daten
status: accepted
tags: [domain, profile]
---
# Kontext

Der Plan fordert Profile als eingebettete YAML-Dateien in `internal/domain/profile`, zugleich aber eine Domäne, die nur die Standardbibliothek importiert. Beides zusammen geht nicht, weil die Standardbibliothek kein YAML parsen kann.

# Entscheidung

Profile und Agent-Registry sind Go-Literale in der Domäne. Erweiterungen liest `configfile` aus `~/.boxai/config.yaml`.

# Konsequenzen

Die Daten sind typgeprüft und ohne Ladefehler; eine Änderung an mitgelieferten Profilen erfordert einen neuen Build.
