---
type: DomainConcept
title: Workspace
status: active
tags: [domain, git]
---
# Modell

`internal/domain/workspace` plant Pfade, Branches und Namen ohne I/O: `<root>/<name>/<repo>`, Sandbox `boxai-<name>`.

# Branch-Regeln

- Standard: neuer Branch `boxai/<name>`, überschreibbar mit `--branch`.
- Base: Repo-Eintrag vor `--base` vor Default-Branch des Repos.
- `--checkout` nutzt einen bestehenden Branch und schließt `--branch` aus.

# Remote-Repos

`ParseRemote` zerlegt HTTPS-, `ssh://`- und scp-artige URLs inklusive verschachtelter GitLab-Gruppen. Der Clone liegt unter `~/.boxai/repos/<host>/<owner>/<repo>`.
