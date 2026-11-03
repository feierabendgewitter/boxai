---
type: Decision
title: Hexagonale Architektur
status: accepted
tags: [architektur]
---
# Kontext

Viele sbx-Features sind experimentell und ändern sich schnell; der Spike hat das bestätigt.

# Entscheidung

Domäne und Use Cases definieren Ports, Adapter implementieren sie. Details stehen unter [Hexagonale Architektur](../architecture/hexagonal.md).

# Konsequenzen

sbx-Änderungen bleiben in einem Adapter, und Use Cases lassen sich vollständig gegen Fakes testen.
