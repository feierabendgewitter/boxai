---
type: Glossary
title: Glossar
status: active
tags: [begriffe]
---
# Begriffe

- **Workspace:** Ordner `<root>/<name>/` mit je einem Worktree pro Repo; primärer Mount der Sandbox.
- **Worktree:** Mit `git worktree add` erzeugter Arbeitsbaum. Seine `.git`-Datei zeigt per absolutem Pfad ins Haupt-Repo.
- **Haupt-Repo:** Das Repo, aus dem ein Worktree stammt; sein `.git` wird zusätzlich gemountet.
- **Sandbox:** sbx-microVM mit eigenem Kernel, Docker-Daemon und Proxy. boxai benennt sie `boxai-<workspace>`.
- **Kit:** Deklaratives sbx-Artefakt. `workload` liefert das Dateisystem, `mixin` ergänzt es.
- **Template:** Container-Image, das über `sbx create -t` das Image eines Agents ersetzt.
- **Platzhalter:** Wert, den die Sandbox statt eines Secrets sieht; der Proxy ersetzt ihn im Request an erlaubte Hosts.
