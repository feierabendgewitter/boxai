# Änderungshistorie

## 2026-11-06

`boxai agent-help` ergänzt: gibt eine eingebettete Anleitung für bedienende KI-Agenten und eine aus dem Kommandobaum generierte Referenz aus.

## 2026-11-04

**Phase 10:** README, Beispiele, GoReleaser, Runbooks. `doctor --network` ergänzt. Die Abnahme auf einer frischen Maschine steht aus.

**Phase 9:** Copilot-Mappings und Antigravity-Kit in echten Sandboxes geprüft.

**Phase 8:** Images und Profile; Spring Boot und React bauen in der Sandbox. `boxai rm` entfernt nun auch die Kit-Images.

**Phasen 4 bis 7:** Use Cases `CreateWorkspace` (Rollback, Idempotenz), `RemoveWorkspace`, `lifecycle`; Adapter `statejson`, `configfile`, `assetfs`, `kitfs`, `sbxcli`, `dockerbuild`; CLI vollständig; E2E-Suite gegen echtes sbx. Die Phasen hängen über den Composition Root zusammen und stehen deshalb in einem Commit.

## 2026-11-03

**Phase 3:** Ports, Fakes (`porttest`), Adapter `fsscan` und `gitcli`, Use Case `ScanRepos`, `boxai scan`. Der Architekturtest liest nun die Quellen selbst, weil ein Cache-Treffer von `go test` einen Verstoß sonst verbarg.

**Phase 2:** Domäne umgesetzt: Workspace-Planung, Agent-Registry, Konfliktregel, Secret-Validierung, Profile, KitSpec.

**Phase 1:** Gerüst, `execx`-Runner, `boxai doctor`, Architekturtest, CI und dieses Bundle angelegt.

**Phase 0:** sbx v0.51.0 erkundet; Ergebnisse in [Reales Verhalten von sbx](findings/sbx-verhalten.md).
