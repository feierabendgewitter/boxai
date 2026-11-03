---
okf_version: "0.2"
---
# boxai

`boxai` bündelt mehrere Git-Repos als Worktrees und startet sie in einer Docker Sandbox.

## Überblick

- [Überblick](overview.md): Ziel, Scope und Nicht-Ziele.
- [Glossar](glossary.md): Workspace, Worktree, Kit, Template, Sandbox.
- [Änderungshistorie](log.md): was sich wann geändert hat.

## Architektur

- [Hexagonale Architektur](architecture/hexagonal.md): Schichten und Abhängigkeitsregel.
- [Ports](architecture/ports.md): alle Ports mit Verantwortung.
- [execx](architecture/adapters/execx.md): Subprozess-Runner mit Dry-Run und Redaction.
- [gitcli](architecture/adapters/gitcli.md): Git-Operationen über das Binary.
- [fsscan](architecture/adapters/fsscan.md): findet Repos im Dateisystem.
- [kitfs](architecture/adapters/kitfs.md): schreibt den v3-Mixin.
- [assetfs](architecture/adapters/assetfs.md): liest Agent- und Skill-Dateien, erkennt OKF-Bundles.
- [sbxcli](architecture/adapters/sbxcli.md): Sandboxes und Secrets über `sbx`.
- [dockerbuild](architecture/adapters/dockerbuild.md): baut und lädt Images.
- [configfile](architecture/adapters/configfile.md): `boxai.yaml` und Secret-Dateien.
- [statejson](architecture/adapters/statejson.md): Workspace-Zustand.

## Domäne

- [Workspace](domain/workspace.md): Pfade, Branches, Remote-URLs.
- [Agent-Registry](domain/agent-registry.md): Asset-Mappings und Konfliktregel.
- [Secrets](domain/secrets.md): Platzhalter-Modell und Redaction.
- [Stack-Profile](domain/profiles.md): `kotlin-spring`, `react`, Image-Wahl.

## Entscheidungen

- [0001 sbx als Subprozess](decisions/0001-sbx-als-subprozess.md)
- [0002 git-CLI statt go-git](decisions/0002-git-cli-statt-go-git.md)
- [0003 Hexagonale Architektur](decisions/0003-hexagonale-architektur.md)
- [0004 Kits v3](decisions/0004-kits-v3.md): ersetzt die Plan-Annahme Kits v2.
- [0005 Worktree-Mounts](decisions/0005-worktree-mounts.md): `.git` der Haupt-Repos als Zusatz-Workspaces.
- [0006 Eigener OKF-Linter](decisions/0006-okf-linter.md)
- [0007 Profile und Registry als Go-Daten](decisions/0007-domain-daten-als-go.md)

## Runbooks

- [Images bauen](runbooks/images-bauen.md)

## Erkenntnisse

- [Reales Verhalten von sbx](findings/sbx-verhalten.md): verifizierte Fakten und Fallstricke; Vorrang vor dem Plan.
