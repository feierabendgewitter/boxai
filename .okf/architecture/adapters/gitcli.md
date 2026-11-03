---
type: Adapter
title: gitcli
status: active
tags: [adapter]
---
# Verantwortung

`internal/adapter/out/gitcli` implementiert `port.GitRepository`.

# Externe Abhängigkeit

`git`-Binary über [execx](execx.md).

# Fallstricke

Lesende Kommandos tragen `ReadOnly` und laufen auch im Dry-Run. `DefaultBranch` versucht `origin/HEAD`, dann `main` und `master`, dann den aktuellen Branch. `CommonDir` löst Symlinks auf, damit der Mount zum Pfad in der `.git`-Datei passt; siehe [Worktree-Mounts](../../decisions/0005-worktree-mounts.md).
