---
type: Runbook
title: Release
status: active
tags: [release]
---
# Voraussetzung

`make check` ist grün und `make e2e` lief lokal, weil CI kein `sbx` hat.

# Ablauf

1. Mindestversion prüfen: Gegen welche sbx-Version lief der E2E? Falls neuer und Verhalten geändert: `MinSbxVersion` in `internal/app/doctor` und [die Erkenntnisse](../findings/sbx-verhalten.md) anpassen.
2. Tag setzen und pushen: `git tag v0.x.y && git push origin v0.x.y`.
3. `.github/workflows/release.yml` baut mit GoReleaser die Archive für Linux und macOS (amd64, arm64).

# Lokal prüfen

`goreleaser check` und `goreleaser release --snapshot --clean`.
