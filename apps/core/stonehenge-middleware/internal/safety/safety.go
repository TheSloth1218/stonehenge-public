package safety

import (
	"fmt"
	"time"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
)

type Controller interface {
	Name() string
	EvaluateFirePermissive(gun model.GunState, target model.Fragment) model.SafetyDecision
	GetHardLimits() model.HardLimits
	GetSpecialMeasuresState() model.SpecialMeasuresState
	Health() model.SafetyHealth
}

type SimulatedController struct {
	hardLimits      model.HardLimits
	specialMeasures model.SpecialMeasuresState
}

func NewSimulatedController(cfg config.SafetyConfig) *SimulatedController {
	return &SimulatedController{
		hardLimits: model.HardLimits{
			MinElevation: cfg.HardMinElevation,
			MaxElevation: cfg.HardMaxElevation,
		},
		specialMeasures: model.SpecialMeasuresState{
			Enabled: cfg.SpecialMeasuresEnabled,
			Reason:  "configured at startup only in v1",
		},
	}
}

func (c *SimulatedController) Name() string {
	return "simulated"
}

func (c *SimulatedController) Health() model.SafetyHealth {
	return model.SafetyHealth{Name: c.Name(), Healthy: true}
}

func (c *SimulatedController) GetHardLimits() model.HardLimits {
	return c.hardLimits
}

func (c *SimulatedController) GetSpecialMeasuresState() model.SpecialMeasuresState {
	return c.specialMeasures
}

func (c *SimulatedController) EvaluateFirePermissive(gun model.GunState, target model.Fragment) model.SafetyDecision {
	decision := model.SafetyDecision{
		Allowed:         true,
		Reason:          "permissive granted",
		HardLimitStatus: "OK",
		Timestamp:       time.Now().UTC(),
	}
	if gun.TargetElevation < c.hardLimits.MinElevation || gun.TargetElevation > c.hardLimits.MaxElevation {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("target elevation %.2f outside hard safety limits %.2f..%.2f", gun.TargetElevation, c.hardLimits.MinElevation, c.hardLimits.MaxElevation)
		decision.HardLimitStatus = "ELEVATION_HARD_LIMIT_VIOLATION"
		return decision
	}
	if target.Elevation < c.hardLimits.MinElevation || target.Elevation > c.hardLimits.MaxElevation {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("fragment elevation %.2f outside hard safety limits %.2f..%.2f", target.Elevation, c.hardLimits.MinElevation, c.hardLimits.MaxElevation)
		decision.HardLimitStatus = "TARGET_HARD_LIMIT_VIOLATION"
		return decision
	}
	if gun.ShellType == model.ShellSpecialLargeFragment && !c.specialMeasures.Enabled {
		decision.Allowed = false
		decision.Reason = "SPECIAL_LARGE_FRAGMENT requires special measures enabled"
		decision.HardLimitStatus = "SPECIAL_MEASURES_REQUIRED"
		decision.SpecialMeasuresRequired = true
		return decision
	}
	return decision
}
