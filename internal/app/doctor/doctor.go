// Package doctor implementiert den Use Case `boxai doctor`.
package doctor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

// MinSbxVersion ist die älteste Version, gegen die boxai verifiziert wurde.
const MinSbxVersion = "0.51.0"

// Service implementiert port.Doctor.
type Service struct {
	probe port.HostProbe
}

// New erzeugt den Use Case.
func New(probe port.HostProbe) *Service { return &Service{probe: probe} }

// Run implementiert port.Doctor.
func (s *Service) Run(ctx context.Context, req port.DoctorRequest) port.DoctorReport {
	var report port.DoctorReport
	add := func(c port.DoctorCheck) { report.Checks = append(report.Checks, c) }

	add(s.toolCheck(ctx, "git", true))

	sbx := s.probe.Tool(ctx, "sbx")
	check := port.DoctorCheck{Name: "sbx", Required: true, OK: sbx.Found, Detail: detailOf(sbx)}
	if sbx.Found && !AtLeast(sbx.Version, MinSbxVersion) {
		check.OK = false
		check.Detail = fmt.Sprintf("Version %s, benötigt mindestens %s (brew upgrade --cask sbx)", sbx.Version, MinSbxVersion)
	}
	add(check)

	if sbx.Found {
		ok, detail := s.probe.SbxAuthenticated(ctx)
		if !ok && detail == "" {
			detail = "nicht angemeldet (sbx login)"
		}
		add(port.DoctorCheck{Name: "sbx login", Required: true, OK: ok, Detail: detail})
	}
	add(s.toolCheck(ctx, "docker", req.NeedDocker))
	return report
}

func (s *Service) toolCheck(ctx context.Context, name string, required bool) port.DoctorCheck {
	info := s.probe.Tool(ctx, name)
	return port.DoctorCheck{Name: name, Required: required, OK: info.Found, Detail: detailOf(info)}
}

func detailOf(info port.ToolInfo) string {
	if !info.Found {
		if info.Detail != "" {
			return info.Detail
		}
		return "nicht gefunden"
	}
	return info.Version
}

// AtLeast vergleicht Versionen numerisch; ein führendes v wird ignoriert.
func AtLeast(version, minimum string) bool {
	have, want := parse(version), parse(minimum)
	for i := range want {
		if have[i] != want[i] {
			return have[i] > want[i]
		}
	}
	return true
}

func parse(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	for i, part := range strings.SplitN(v, ".", 3) {
		digits := strings.TrimRightFunc(part, func(r rune) bool { return r < '0' || r > '9' })
		out[i], _ = strconv.Atoi(digits)
	}
	return out
}
