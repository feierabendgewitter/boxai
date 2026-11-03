---
type: Adapter
title: fsscan
status: active
tags: [adapter]
---
# Verantwortung

`internal/adapter/out/fsscan` implementiert `port.RepoScanner`.

# Externe Abhängigkeit

Dateisystem über `filepath.WalkDir`.

# Fallstricke

`.git`-Ordner bedeutet Repo, `.git`-Datei bedeutet Worktree, `HEAD` plus `objects` plus `refs` bedeutet Bare-Repo. Innerhalb eines Repos wird nicht weitergesucht. Git versioniert keine `.git`-Ordner, deshalb nutzt das Fixture `dot-git`-Namen, die der Test umbenennt.
