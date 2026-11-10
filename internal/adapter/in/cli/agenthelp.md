# boxai für KI-Agenten

boxai läuft auf dem Host. Es bündelt Git-Repos als Worktrees zu einem Workspace und startet darin einen Agenten in einer Docker Sandbox (`sbx`). Diese Anleitung gilt für Agenten, die boxai auf dem Host bedienen.

## Ablauf

1. `boxai doctor` prüft die Voraussetzungen.
2. `boxai scan --dir <ordner>` zeigt, welche Repos verfügbar sind.
3. `boxai up --dry-run ...` zeigt alle geplanten git- und sbx-Kommandos, ohne etwas zu ändern.
4. `boxai up --no-attach ...` legt Worktrees und Sandbox an, startet den Agenten aber nicht.
5. `boxai list` zeigt Workspaces und den Status ihrer Sandboxes.
6. `boxai stop <name>` hält die Sandbox an, `boxai rm <name>` entfernt den Workspace.

Beispiel:

```sh
boxai up --no-attach --dir ~/projekte --name shop --repo backend --repo frontend --agent claude
```

## Ohne Terminal aufrufen

- Setze `--name`, `--repo` und `--agent` oder nutze `--config`. Sonst fragt `boxai up` nach und bricht ohne Terminal mit einem Fehler ab.
- `boxai up` ohne `--no-attach` und `boxai attach` übernehmen das Terminal und haben kein Zeitlimit. Sie blockieren, bis der Agent endet. Überlasse sie einem Menschen, wenn du kein echtes Terminal hast.
- Ein erneuter Lauf von `boxai up` setzt einen bestehenden Workspace fort.
- `boxai init` schreibt Vorlagen für `boxai.yaml` und `secrets.example.yaml`.

## Ausgabe und Exit-Codes

- Ergebnisse stehen auf stdout, Hinweise und Fehler auf stderr.
- Exit-Code 0 bedeutet Erfolg.
- Exit-Code 1 bedeutet einen Fehler oder eine fehlgeschlagene Pflichtprüfung von `boxai doctor`.
- `boxai up` und `boxai attach` reichen den Exit-Code des gestarteten Agenten durch, sobald er läuft.

## Sicherheitsregeln

- Nutze `boxai rm --force` und `--delete-branches` nur auf ausdrücklichen Auftrag. Sie können uncommittete Arbeit und Branches löschen.
- Referenziere Secrets in Secret-Dateien mit `${env:NAME}` oder `${file:pfad}`. Schreibe Secret-Werte nie im Klartext in Dateien oder Kommandozeilen.
- Der Agent in der Sandbox kann über den Mount von `<repo>/.git` Objekte und Branches der Haupt-Repos verändern. Prüfe seine Ergebnisse, bevor du sie übernimmst.
