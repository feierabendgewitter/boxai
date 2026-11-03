# Änderungshistorie

## 2026-11-03

**Phase 3:** Ports, Fakes (`porttest`), Adapter `fsscan` und `gitcli`, Use Case `ScanRepos`, `boxai scan`. Der Architekturtest liest nun die Quellen selbst, weil ein Cache-Treffer von `go test` einen Verstoß sonst verbarg.

**Phase 2:** Domäne umgesetzt: Workspace-Planung, Agent-Registry, Konfliktregel, Secret-Validierung, Profile, KitSpec.

**Phase 1:** Gerüst, `execx`-Runner, `boxai doctor`, Architekturtest, CI und dieses Bundle angelegt.

**Phase 0:** sbx v0.51.0 erkundet; Ergebnisse in [Reales Verhalten von sbx](findings/sbx-verhalten.md).
