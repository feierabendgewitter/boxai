---
type: Architecture
title: Ports
status: active
tags: [architektur, ports]
---
# Outbound-Ports

Definiert in `internal/app/port/out.go`, Fakes in `internal/app/port/porttest`.

| Port | Verantwortung | Adapter |
| --- | --- | --- |
| `HostProbe` | Tools und sbx-Login prüfen | `hostprobe` |
| `RepoScanner` | Repos im Dateisystem finden | [fsscan](adapters/fsscan.md) |
| `GitRepository` | Branches, Clone, Worktrees, Common-Dir | [gitcli](adapters/gitcli.md) |
| `SandboxRuntime` | Sandboxes erzeugen, starten, entfernen; Netzwerk freigeben | [sbxcli](adapters/sbxcli.md) |
| `SecretStore` | Secrets pro Sandbox hinterlegen und löschen | [sbxcli](adapters/sbxcli.md) |
| `KitWriter` | Mixin schreiben und prüfen, Built-in-Kits bereitstellen | [kitfs](adapters/kitfs.md) |
| `AssetSource` | Agent- und Skill-Dateien sowie OKF-Bundles lesen | [assetfs](adapters/assetfs.md) |
| `ImageBuilder` | Images bauen, laden, prüfen | [dockerbuild](adapters/dockerbuild.md) |
| `ConfigLoader` | `boxai.yaml`, globale Konfiguration, Secrets | [configfile](adapters/configfile.md) |
| `StateStore` | Workspace-Zustand | [statejson](adapters/statejson.md) |
| `Prompter`, `Notifier` | Interaktive Auswahl, Meldungen | `adapter/in/cli` |

Gegenüber der Skizze im Plan hinzugekommen: `BranchExists` und `PruneWorktrees` in `GitRepository`, `Remove`, `RemoveWorkspaceRoot` und `Builtin` in `KitWriter`, `Available` und `AgentLabel` in `ImageBuilder`, `RepoContext` in `AssetSource`, `LoadGlobalConfig` und `NormalizePath` in `ConfigLoader`, `HostProbe`, `Notifier`.

# Inbound-Ports

Definiert in `internal/app/port/in.go`: `Doctor`, `RepoScan`, `WorkspaceCreator`, `WorkspaceRemover`, `Lifecycle`. Das CLI ruft pro Kommando genau einen auf.
