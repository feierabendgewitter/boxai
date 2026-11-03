// Package kits bettet die mitgelieferten Sandbox-Kits in das Binary ein.
package kits

import "embed"

// FS enthält je Kit einen Ordner.
//
//go:embed antigravity
var FS embed.FS
