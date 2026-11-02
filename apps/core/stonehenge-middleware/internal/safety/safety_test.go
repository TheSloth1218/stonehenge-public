package safety

import (
	"testing"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
)

func TestSafetyPermissiveBlocksUnsafeElevation(t *testing.T) {
	controller := NewSimulatedController(config.SafetyConfig{HardMinElevation: 0, HardMaxElevation: 90})
	gun := model.GunState{TargetElevation: 91, ShellType: model.ShellStandard}
	target := model.Fragment{Elevation: 91}
	decision := controller.EvaluateFirePermissive(gun, target)
	if decision.Allowed {
		t.Fatal("expected hard limit violation to block fire")
	}
	if decision.HardLimitStatus != "ELEVATION_HARD_LIMIT_VIOLATION" {
		t.Fatalf("unexpected hard limit status %q", decision.HardLimitStatus)
	}
}

func TestSpecialShellBlockedUnlessSpecialMeasuresEnabled(t *testing.T) {
	controller := NewSimulatedController(config.SafetyConfig{HardMinElevation: 0, HardMaxElevation: 90, SpecialMeasuresEnabled: false})
	gun := model.GunState{TargetElevation: 45, ShellType: model.ShellSpecialLargeFragment}
	target := model.Fragment{Elevation: 45}
	decision := controller.EvaluateFirePermissive(gun, target)
	if decision.Allowed {
		t.Fatal("expected special shell to be blocked without special measures")
	}
	if !decision.SpecialMeasuresRequired {
		t.Fatal("expected special measures required flag")
	}

	controller = NewSimulatedController(config.SafetyConfig{HardMinElevation: 0, HardMaxElevation: 90, SpecialMeasuresEnabled: true})
	decision = controller.EvaluateFirePermissive(gun, target)
	if !decision.Allowed {
		t.Fatalf("expected special shell to be allowed with special measures enabled: %s", decision.Reason)
	}
}
