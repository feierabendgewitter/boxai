---
type: Decision
title: .git der Haupt-Repos als Zusatz-Workspaces
status: accepted
tags: [sbx, git]
---
# Kontext

Die `.git`-Datei eines Worktrees enthält einen absoluten Host-Pfad ins Haupt-Repo. Fehlt dieser Pfad in der Sandbox, scheitert jedes Git-Kommando.

# Entscheidung

boxai mountet je Haupt-Repo das Git-Common-Dir (`<repo>/.git`) read-write unter seinem Host-Pfad. Der [Spike](../findings/sbx-verhalten.md) zeigt: Das genügt für Commits. Das restliche Haupt-Repo bleibt außerhalb der Sandbox.

# Konsequenzen

Der Agent kann Objekte und Branches des Host-Repos verändern, sieht aber dessen Arbeitsbaum nicht. Pfade werden mit `filepath.Abs` und `EvalSymlinks` normalisiert, damit sie mit den absoluten Pfaden in der `.git`-Datei übereinstimmen.
