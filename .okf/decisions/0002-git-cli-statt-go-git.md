---
type: Decision
title: git-CLI statt go-git
status: accepted
tags: [git, adapter]
---
# Kontext

go-git unterstützt bei Worktrees nur `add` vollständig; die API ist experimentell.

# Entscheidung

Der Adapter `internal/adapter/out/gitcli` ruft das `git`-Binary über `execx` auf.

# Konsequenzen

`git` ist eine Laufzeitabhängigkeit, die `boxai doctor` prüft. Verhalten und Konfiguration entsprechen dem, was Nutzer von Git kennen.
