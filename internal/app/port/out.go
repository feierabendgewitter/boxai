// Package port definiert die Inbound- und Outbound-Ports des Anwendungskerns.
package port

import "context"

// ToolInfo beschreibt den Fund eines Host-Tools.
type ToolInfo struct {
	Found   bool
	Version string
	Detail  string
}

// HostProbe prüft die Host-Umgebung für `boxai doctor`.
type HostProbe interface {
	Tool(ctx context.Context, name string) ToolInfo
	// SbxAuthenticated meldet, ob `sbx` bei Docker angemeldet ist.
	SbxAuthenticated(ctx context.Context) (bool, string)
}
