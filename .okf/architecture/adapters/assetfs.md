---
type: Adapter
title: assetfs
status: active
tags: [adapter, agent]
---
# Verantwortung

`internal/adapter/out/assetfs` implementiert `port.AssetSource`: Es liest Agent- und Skill-Dateien nach den Mappings der [Registry](../../domain/agent-registry.md) und erkennt OKF-Bundles sowie Anweisungsdateien.

# Externe Abhängigkeit

Dateisystem.

# Fallstricke

Dateien landen inline im [Kit-Deskriptor](kitfs.md). Binärdateien, Symlinks und Dateien über 1 MiB werden deshalb übersprungen und gemeldet. Ein OKF-Bundle gilt erst mit `index.md` plus einem Konzept mit `type`-Frontmatter als erkannt, damit gewöhnliche `docs/index.md` nicht zählen.
