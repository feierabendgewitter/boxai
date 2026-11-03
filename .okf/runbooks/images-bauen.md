---
type: Runbook
title: Images bauen
status: active
tags: [images]
---
# Voraussetzungen

Docker läuft, `sbx` ist angemeldet (`boxai doctor --docker`).

# Bauen

`make images` baut alle Varianten für Claude und Copilot und lädt sie mit `sbx template load`. Eine einzelne Variante: `images/build.sh claude kotlin`, ohne Docker Engine zusätzlich `--no-docker`. Fehlende Basis-Ebenen entstehen automatisch.

| Tag | Dockerfile | Basis |
| --- | --- | --- |
| `boxai-base-<agent>` | `images/base.Dockerfile` | `docker/sandbox-templates:<variante>-docker` |
| `boxai-kotlin-<agent>` | `images/kotlin.Dockerfile` | `boxai-base-<agent>` |
| `boxai-react-<agent>` | `images/react.Dockerfile` | `boxai-base-<agent>` |
| `boxai-fullstack-<agent>` | `images/react.Dockerfile` | `boxai-kotlin-<agent>` |

Rechne mit etwa 4 GB pro Image im Docker-Store und in der sbx-Runtime. Versionen stehen als `ARG` in den Dockerfiles; ihre URLs prüfen, bevor du sie erhöhst.

# Prüfen

`sbx template ls | grep boxai-` listet die geladenen Images.
