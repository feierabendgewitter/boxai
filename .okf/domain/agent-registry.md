---
type: DomainConcept
title: Agent-Registry
status: active
tags: [domain, agent]
---
# Zweck

`internal/domain/agent` hält als Daten, wie boxai jeden Agent startet und welche Dateien aus den Repos in die Sandbox gelangen. `~/.boxai/config.yaml` kann zusätzliche Mappings ergänzen.

# Asset-Mappings

| Agent | Quelle im Repo | Ziel |
| --- | --- | --- |
| Claude Code | `.claude/agents/*.md` | `~/.claude/agents/` |
| Claude Code | `.claude/skills/<n>/` | `~/.claude/skills/<n>/` |
| Claude Code | `.claude/commands/*.md` | `~/.claude/commands/` |
| Copilot CLI | `.github/agents/*.md` | `~/.copilot/agents/` |
| Copilot CLI | `.github/skills/<n>/`, `.claude/skills/<n>/` | `~/.copilot/skills/<n>/` |
| Antigravity | `.agents/skills/<n>/` | `<workspace>/.agents/skills/<n>/` |

`AGENTS.md`, `CLAUDE.md`, `.github/copilot-instructions.md` und OKF-Bundles bleiben im Repo; die generierte Root-`AGENTS.md` verweist darauf.

# Konfliktregel

Gleichnamige Assets aus verschiedenen Repos werden alle als `<repo>-<name>` abgelegt und gemeldet. Doppelte Assets aus demselben Repo (etwa ein Skill unter `.github/skills` und `.claude/skills`) behalten das zuerst gemappte und melden das zweite.

# Reservierte Pfade

Von sbx verwaltet und nie beschrieben: `~/.claude.json`, `~/.claude/settings.json`, `~/.claude/.config.json`, `~/.copilot/config.json`.

# Antigravity

Antigravity ist kein Built-in-Agent und läuft über den eigenen v3-Workload in `kits/antigravity`, den `kits/embed.go` ins Binary einbettet. Es ist als experimentell gekennzeichnet. Die CLI heißt `agy`; Details zur Anmeldung stehen in den [Erkenntnissen](../findings/sbx-verhalten.md).

Der Workload bringt sein Image selbst mit. Ein Profil-Image würde `agy` ersetzen, deshalb fehlen dort die Tools der Profile; Netzwerk und Konfiguration der Profile gelten weiterhin.
