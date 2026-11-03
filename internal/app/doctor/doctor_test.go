package doctor

import (
	"context"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

type fakeProbe struct {
	tools map[string]port.ToolInfo
	auth  bool
}

func (f fakeProbe) Tool(_ context.Context, name string) port.ToolInfo { return f.tools[name] }
func (f fakeProbe) SbxAuthenticated(context.Context) (bool, string)   { return f.auth, "" }

func find(t *testing.T, r port.DoctorReport, name string) port.DoctorCheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("Prüfung %s fehlt", name)
	return port.DoctorCheck{}
}

func healthyTools() map[string]port.ToolInfo {
	return map[string]port.ToolInfo{
		"git":    {Found: true, Version: "2.54.0"},
		"sbx":    {Found: true, Version: "0.51.0"},
		"docker": {Found: true, Version: "29.9.2"},
	}
}

func TestHealthy(t *testing.T) {
	r := New(fakeProbe{tools: healthyTools(), auth: true}).Run(context.Background(), port.DoctorRequest{NeedDocker: true})
	if !r.Healthy() {
		t.Fatalf("%+v", r)
	}
}

func TestMissingToolsAreReported(t *testing.T) {
	tests := []struct {
		name    string
		missing string
		req     port.DoctorRequest
		healthy bool
	}{
		{"git fehlt", "git", port.DoctorRequest{}, false},
		{"sbx fehlt", "sbx", port.DoctorRequest{}, false},
		{"docker optional", "docker", port.DoctorRequest{}, true},
		{"docker gefordert", "docker", port.DoctorRequest{NeedDocker: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := healthyTools()
			tools[tt.missing] = port.ToolInfo{}
			r := New(fakeProbe{tools: tools, auth: true}).Run(context.Background(), tt.req)
			if c := find(t, r, tt.missing); c.OK {
				t.Fatalf("%s müsste fehlen", tt.missing)
			}
			if r.Healthy() != tt.healthy {
				t.Fatalf("Healthy = %v", r.Healthy())
			}
		})
	}
}

func TestOldSbxAndMissingLoginFail(t *testing.T) {
	tools := healthyTools()
	tools["sbx"] = port.ToolInfo{Found: true, Version: "v0.46.0"}
	r := New(fakeProbe{tools: tools, auth: true}).Run(context.Background(), port.DoctorRequest{})
	if find(t, r, "sbx").OK {
		t.Fatal("alte sbx-Version akzeptiert")
	}
	r = New(fakeProbe{tools: healthyTools(), auth: false}).Run(context.Background(), port.DoctorRequest{})
	if find(t, r, "sbx login").OK || r.Healthy() {
		t.Fatal("fehlender Login akzeptiert")
	}
}

func TestAtLeast(t *testing.T) {
	tests := []struct {
		have, want string
		ok         bool
	}{
		{"0.51.0", "0.51.0", true},
		{"v0.52.1", "0.51.0", true},
		{"1.0.0-rc1", "0.51.0", true},
		{"0.9.9", "0.51.0", false},
		{"", "0.51.0", false},
	}
	for _, tt := range tests {
		if got := AtLeast(tt.have, tt.want); got != tt.ok {
			t.Errorf("AtLeast(%q, %q) = %v", tt.have, tt.want, got)
		}
	}
}
