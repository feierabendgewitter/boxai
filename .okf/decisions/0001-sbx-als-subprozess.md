---
type: Decision
title: sbx als Subprozess statt Docker-SDK
status: accepted
tags: [sbx, adapter]
---
# Kontext

Docker Sandboxes laufen als microVMs; das Docker-Engine-SDK erreicht sie nicht.

# Entscheidung

Der Adapter `internal/adapter/out/sbxcli` ruft das `sbx`-Binary auf.

# Konsequenzen

CLI-Änderungen betreffen nur diesen Adapter; die Mindestversion prüft `boxai doctor`.
