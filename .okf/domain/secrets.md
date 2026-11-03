---
type: DomainConcept
title: Secrets
status: active
tags: [domain, secrets]
---
# Platzhalter-Modell

Die Sandbox sieht in der Env-Variable nur einen Platzhalter. Der sbx-Proxy ersetzt ihn in Request-Headern an die erlaubten Hosts. Deshalb sind `hosts` Pflicht, und boxai gibt jeden Secret-Host auch im Netzwerk frei.

# Redaction

`secret.SecretSpec` maskiert den Wert in `String`, `GoString` und `slog`; `MarshalJSON` schlägt immer fehl, damit kein Wert in den State gelangt. `execx` entfernt registrierte Werte zusätzlich aus allen Ausgaben. Der Wert erreicht sbx nur über Stdin ([Details](../findings/sbx-verhalten.md)).

# Fallback `secrets.mode: env`

Für Tools, bei denen Platzhalter scheitern, etwa AWS-SigV4 (die Signatur entsteht lokal aus dem Platzhalter). boxai übergibt die echten Werte mit `sbx create -e NAME` aus der Prozessumgebung, also nie als Argument, und warnt deutlich. Die Sandbox sieht dann den Klartext.

# Referenzen

`${env:NAME}` und `${file:pfad}` in YAML, `${NAME}` zusätzlich in `.env`. Die Domäne parst, der Adapter `configfile` löst auf.
