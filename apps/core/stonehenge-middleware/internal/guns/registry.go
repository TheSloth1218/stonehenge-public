package guns

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/plc"
)

type Definition struct {
	ID                  int     `json:"id"`
	Name                string  `json:"name"`
	PLCEnabled          bool    `json:"plc_enabled"`
	SafetyGunID         int     `json:"safety_gun_id"`
	AutoEnabled         bool    `json:"auto_enabled"`
	DispatcherEligible  bool    `json:"dispatcher_eligible"`
	AutoExecutorEnabled bool    `json:"auto_executor_enabled"`
	MinAzimuth          float64 `json:"min_azimuth"`
	MaxAzimuth          float64 `json:"max_azimuth"`
	MinElevation        float64 `json:"min_elevation"`
	MaxElevation        float64 `json:"max_elevation"`
	PLC                 config.PLCConfig
}

type Instance struct {
	Definition  Definition
	Commands    *plc.GunCommandService
	Lifecycle   *plc.CommandLifecycleTracker
	Auto        *autocontrol.Controller
	Arbitration *arbitration.Service
}

type Registry struct {
	mu      sync.RWMutex
	ordered []*Instance
	byID    map[int]*Instance
}

func NewRegistry(instances []*Instance) (*Registry, error) {
	r := &Registry{byID: make(map[int]*Instance, len(instances))}
	for _, inst := range instances {
		if inst == nil {
			return nil, errors.New("gun registry contains nil instance")
		}
		id := inst.Definition.ID
		if id <= 0 {
			return nil, fmt.Errorf("gun id must be positive: %d", id)
		}
		if _, exists := r.byID[id]; exists {
			return nil, fmt.Errorf("duplicate gun id %d", id)
		}
		r.byID[id] = inst
		r.ordered = append(r.ordered, inst)
	}
	sort.Slice(r.ordered, func(i, j int) bool {
		return r.ordered[i].Definition.ID < r.ordered[j].Definition.ID
	})
	return r, nil
}

func Build(defs []Definition, commandAuditor history.CommandAuditor, engagementAuditor history.EngagementAuditor, eventSink history.Sink, logger *slog.Logger) (*Registry, error) {
	if len(defs) == 0 {
		return nil, errors.New("at least one gun definition is required")
	}
	instances := make([]*Instance, 0, len(defs))
	for _, def := range defs {
		if def.ID <= 0 {
			return nil, fmt.Errorf("gun id must be positive: %d", def.ID)
		}
		if def.Name == "" {
			def.Name = fmt.Sprintf("Gun %d", def.ID)
		}
		commands := plc.NewGunCommandServiceForGun(def.ID, def.PLC, logger)
		arbitrator := arbitration.NewServiceForGun(def.ID, commands, logger)
		var lifecycle *plc.CommandLifecycleTracker
		if commandAuditor != nil {
			lifecycle = plc.NewCommandLifecycleTracker(def.PLC, commands, commandAuditor, logger)
		}
		auto := autocontrol.NewControllerForGun(def.ID, def.PLC, commands, lifecycle, commandAuditor, engagementAuditor, eventSink, logger)
		auto.SetArbitrator(arbitrator)
		auto.SetEnabled(def.AutoEnabled)
		instances = append(instances, &Instance{
			Definition:  def,
			Commands:    commands,
			Lifecycle:   lifecycle,
			Auto:        auto,
			Arbitration: arbitrator,
		})
	}
	return NewRegistry(instances)
}

func (r *Registry) Get(id int) (*Instance, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.byID[id]
	return inst, ok
}

func (r *Registry) MustGet(id int) *Instance {
	inst, ok := r.Get(id)
	if !ok {
		return nil
	}
	return inst
}

func (r *Registry) List() []*Instance {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Instance, len(r.ordered))
	copy(out, r.ordered)
	return out
}

func (r *Registry) StartAll() {
	for _, inst := range r.List() {
		if inst.Auto != nil {
			inst.Auto.Start()
		}
	}
}

func (r *Registry) Close() {
	for _, inst := range r.List() {
		if inst.Auto != nil {
			inst.Auto.Close()
		}
		if inst.Lifecycle != nil {
			inst.Lifecycle.Close()
		}
	}
}
