package port

import "context"

// DoctorRequest wählt optionale Prüfungen.
type DoctorRequest struct {
	NeedDocker bool
}

// DoctorCheck ist das Ergebnis einer Prüfung.
type DoctorCheck struct {
	Name     string
	OK       bool
	Required bool
	Detail   string
}

// DoctorReport fasst alle Prüfungen zusammen.
type DoctorReport struct {
	Checks []DoctorCheck
}

// Healthy meldet, ob alle Pflichtprüfungen bestanden sind.
func (r DoctorReport) Healthy() bool {
	for _, c := range r.Checks {
		if c.Required && !c.OK {
			return false
		}
	}
	return true
}

// Doctor prüft die Voraussetzungen von boxai.
type Doctor interface {
	Run(ctx context.Context, req DoctorRequest) DoctorReport
}
