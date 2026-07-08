package cli

import (
	"errors"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/doctor"
)

func TestInstitutionalFreshnessCheck(t *testing.T) {
	ref := "github.com/org/base@main"
	tests := []struct {
		name       string
		committed  string
		resolved   string
		resolveErr error
		want       doctor.Status
	}{
		{"resolve error → warn", "abc123", "", errors.New("offline"), doctor.StatusWarn},
		{"no snapshot recorded → warn", "", "abc123", nil, doctor.StatusWarn},
		{"up to date → ok", "abc1234def", "abc1234def", nil, doctor.StatusOK},
		{"behind → warn", "old1111", "new2222", nil, doctor.StatusWarn},
		{"local fixture → ok", "", "local", nil, doctor.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := institutionalFreshnessCheck(ref, tt.committed, tt.resolved, tt.resolveErr)
			if got.Status != tt.want {
				t.Errorf("status = %v, want %v (detail=%q)", got.Status, tt.want, got.Detail)
			}
			if tt.want == doctor.StatusWarn && got.Hint == "" {
				t.Error("warning should carry a remediation hint")
			}
		})
	}
}
