package plc

import (
	"fmt"
	"sync"

	"stonehenge-middleware/internal/model"
)

type Adapter interface {
	ReadGunState(gunID int) (model.GunState, error)
	WriteGunSetpoint(gunID int, azimuth float64, elevation float64) error
	WriteGunMode(gunID int, mode string) error
	WriteFireRequest(gunID int) error
	ReadFirePermissive(gunID int) (bool, string, error)
	UpsertGunState(gun model.GunState)
	SetFirePermissive(gunID int, allowed bool, reason string)
	Health() model.PLCHealth
}

type SimulatedAdapter struct {
	mu           sync.RWMutex
	guns         map[int]model.GunState
	permissives  map[int]permissive
	fireRequests map[int]bool
}

type permissive struct {
	allowed bool
	reason  string
}

func NewSimulatedAdapter() *SimulatedAdapter {
	return &SimulatedAdapter{
		guns:         make(map[int]model.GunState),
		permissives:  make(map[int]permissive),
		fireRequests: make(map[int]bool),
	}
}

func (a *SimulatedAdapter) ReadGunState(gunID int) (model.GunState, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	gun, ok := a.guns[gunID]
	if !ok {
		return model.GunState{}, fmt.Errorf("gun %d not found", gunID)
	}
	return gun, nil
}

func (a *SimulatedAdapter) WriteGunSetpoint(gunID int, azimuth float64, elevation float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	gun, ok := a.guns[gunID]
	if !ok {
		return fmt.Errorf("gun %d not found", gunID)
	}
	gun.TargetAzimuth = model.NormalizeAzimuth(azimuth)
	gun.TargetElevation = elevation
	a.guns[gunID] = gun
	return nil
}

func (a *SimulatedAdapter) WriteGunMode(gunID int, mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	gun, ok := a.guns[gunID]
	if !ok {
		return fmt.Errorf("gun %d not found", gunID)
	}
	gun.Mode = model.GunMode(mode)
	a.guns[gunID] = gun
	return nil
}

func (a *SimulatedAdapter) WriteFireRequest(gunID int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.guns[gunID]; !ok {
		return fmt.Errorf("gun %d not found", gunID)
	}
	a.fireRequests[gunID] = true
	return nil
}

func (a *SimulatedAdapter) ReadFirePermissive(gunID int) (bool, string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	p, ok := a.permissives[gunID]
	if !ok {
		return false, "permissive not evaluated", nil
	}
	return p.allowed, p.reason, nil
}

func (a *SimulatedAdapter) UpsertGunState(gun model.GunState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.guns[gun.ID] = gun
}

func (a *SimulatedAdapter) SetFirePermissive(gunID int, allowed bool, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.permissives[gunID] = permissive{allowed: allowed, reason: reason}
}

func (a *SimulatedAdapter) Health() model.PLCHealth {
	return model.PLCHealth{
		Adapter:   "simulated",
		Connected: true,
	}
}
