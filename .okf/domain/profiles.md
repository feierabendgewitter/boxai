---
type: DomainConcept
title: Stack-Profile
status: active
tags: [domain, profile]
---
# Profile

`internal/domain/profile` enthält `kotlin-spring` und `react`, [als Go-Daten](../decisions/0007-domain-daten-als-go.md). Beide sind kombinierbar. Jede Sandbox erlaubt zusätzlich `github.com`, `api.github.com`, `gitlab.com` und `**.amazonaws.com`.

# Konfigurationsdateien

Dateien verweisen auf Env-Variablen, nie auf Werte. `.npmrc` und `settings.xml` entstehen nur, wenn das passende Secret existiert, weil npm an einer nicht ersetzbaren Variable scheitert.

Abweichung vom Plan: `gradle.properties` kann keine Env-Variablen auflösen. Sie enthält daher kein Token; Build-Skripte lesen `System.getenv("MAVEN_TOKEN")`.

# Image-Wahl

`ImageTag` wählt `boxai-{base,kotlin,react,fullstack}-<agent>[-nodocker]:latest`. Siehe [Images bauen](../runbooks/images-bauen.md).
