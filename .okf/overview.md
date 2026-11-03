---
type: Overview
title: Überblick
status: active
tags: [scope]
---
# Ziel

Ein Agent arbeitet repo-übergreifend (etwa Kotlin-Backend und React-Frontend), isoliert vom Host, mit vorkonfigurierten Tools, Secrets sowie den Agent- und Skill-Dateien aus den Repos.

# Scope

Ein statisches Go-Binary für Linux und macOS. Es scannt Repos, legt Worktrees an, erzeugt einen Mixin-Kit und startet eine sbx-Sandbox mit Claude Code, Copilot CLI oder Antigravity.

# Nicht-Ziele (v1)

- Cloud-Sandboxes
- GUI
- Automatisches Erstellen von Merge Requests
