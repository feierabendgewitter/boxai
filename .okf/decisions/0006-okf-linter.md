---
type: Decision
title: Eigener OKF-Linter
status: accepted
tags: [okf, ci]
---
# Kontext

Der Plan nennt `okft` als möglichen Linter und merkt an, dass er gegen OKF v0.1 prüft, die aktuelle Spezifikation aber v0.2 ist. `okft` ließ sich in der Entwicklungsumgebung nicht ausführen, weil das Nachladen von PyPI-Code dort gesperrt war; seine Eignung ist daher ungeprüft.

# Entscheidung

Das Bundle deklariert `okf_version: "0.2"` in `index.md`. `tools/okflint` prüft die Konformitätsregeln von v0.2 und die boxai-Konventionen `title`, `status` und auflösbare Links. CI und `make check` führen ihn aus.

# Konsequenzen

Keine Python-Abhängigkeit in CI. Eine neue OKF-Version erfordert eine Anpassung des Linters.
