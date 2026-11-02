package guns

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/plc"
)

func TestRegistryListOrderAndLookup(t *testing.T) {
	registry, err := NewRegistry([]*Instance{
		{Definition: Definition{ID: 2, Name: "Gun 2"}},
		{Definition: Definition{ID: 1, Name: "Gun 1"}},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	list := registry.List()
	if len(list) != 2 || list[0].Definition.ID != 1 || list[1].Definition.ID != 2 {
		t.Fatalf("list order = %+v", list)
	}
	if _, ok := registry.Get(3); ok {
		t.Fatal("unexpected gun 3")
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	if _, err := NewRegistry([]*Instance{
		{Definition: Definition{ID: 1}},
		{Definition: Definition{ID: 1}},
	}); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestBuildCreatesIndependentArbitrationAndSequences(t *testing.T) {
	cfg := config.PLCConfig{
		Host:                          "gun",
		Port:                          502,
		UnitID:                        1,
		CommandAckTimeout:             time.Second,
		PollInterval:                  time.Millisecond,
		LifecyclePollInterval:         time.Millisecond,
		LifecycleCompletionTimeout:    time.Second,
		LifecycleMaxConsecutiveErrors: 5,
		AutoReadinessPollInterval:     time.Millisecond,
	}
	registry, err := Build([]Definition{
		{ID: 1, Name: "Gun 1", PLC: cfg, PLCEnabled: true, SafetyGunID: 1, AutoEnabled: true},
		{ID: 2, Name: "Gun 2", PLC: cfg, PLCEnabled: true, SafetyGunID: 2, AutoEnabled: true},
	}, nil, nil, nil, slog.Default())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer registry.Close()
	gun1, _ := registry.Get(1)
	gun2, _ := registry.Get(2)
	if status := gun1.Auto.Status(context.Background()); !status.Enabled {
		t.Fatalf("gun1 auto enabled = false, want true")
	}
	if status := gun2.Auto.Status(context.Background()); !status.Enabled {
		t.Fatalf("gun2 auto enabled = false, want true")
	}
	lease, _, err := gun1.Arbitration.AcquireManual(context.Background())
	if err != nil {
		t.Fatalf("gun1 AcquireManual: %v", err)
	}
	defer lease.Release()
	lease2, _, err := gun2.Arbitration.AcquireManual(context.Background())
	if err != nil {
		t.Fatalf("gun2 AcquireManual should be independent: %v", err)
	}
	defer lease2.Release()
	if status := gun1.Arbitration.Status(); !status.ManualActive {
		t.Fatalf("gun1 arbitration = %+v", status)
	}
	if status := gun2.Arbitration.Status(); !status.ManualActive {
		t.Fatalf("gun2 arbitration = %+v", status)
	}
	if _, err := gun1.Commands.Submit(context.Background(), 2, plc.GunCommand{Mode: plc.ModeManual, TargetAzimuth: 10, TargetElevation: 10}); err == nil {
		t.Fatal("gun1 command service accepted gun2 id")
	}
}

func TestBuildRegistersEightGunsInDeterministicOrder(t *testing.T) {
	cfg := config.PLCConfig{
		Host:                          "gun",
		Port:                          502,
		UnitID:                        1,
		CommandAckTimeout:             time.Second,
		PollInterval:                  time.Millisecond,
		LifecyclePollInterval:         time.Millisecond,
		LifecycleCompletionTimeout:    time.Second,
		LifecycleMaxConsecutiveErrors: 5,
		AutoReadinessPollInterval:     time.Millisecond,
	}
	defs := make([]Definition, 0, 8)
	for id := 8; id >= 1; id-- {
		gunCfg := cfg
		if id == 1 {
			gunCfg.Host = "openplc-control-gun1"
		} else {
			gunCfg.Host = fmt.Sprintf("openplc-control-gun%d", id)
		}
		defs = append(defs, Definition{
			ID:          id,
			Name:        fmt.Sprintf("Gun %d", id),
			PLC:         gunCfg,
			PLCEnabled:  true,
			SafetyGunID: id,
			AutoEnabled: true,
		})
	}
	registry, err := Build(defs, nil, nil, nil, slog.Default())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer registry.Close()

	list := registry.List()
	if len(list) != 8 {
		t.Fatalf("registry len = %d, want 8", len(list))
	}
	endpoints := map[string]struct{}{}
	for i, inst := range list {
		id := i + 1
		if inst.Definition.ID != id {
			t.Fatalf("list[%d] id = %d, want %d", i, inst.Definition.ID, id)
		}
		if _, ok := registry.Get(id); !ok {
			t.Fatalf("gun %d not found", id)
		}
		endpoint := fmt.Sprintf("%s:%d/%d", inst.Definition.PLC.Host, inst.Definition.PLC.Port, inst.Definition.PLC.UnitID)
		if _, ok := endpoints[endpoint]; ok {
			t.Fatalf("duplicate endpoint %s", endpoint)
		}
		endpoints[endpoint] = struct{}{}
		status := inst.Auto.Status(context.Background())
		if !status.Enabled {
			t.Fatalf("gun %d auto enabled = false, want true", id)
		}
	}
	if _, ok := registry.Get(9); ok {
		t.Fatal("unexpected gun 9")
	}

	leases := make([]*arbitration.Lease, 0, 8)
	for _, inst := range list {
		lease, _, err := inst.Arbitration.AcquireManual(context.Background())
		if err != nil {
			t.Fatalf("gun %d AcquireManual: %v", inst.Definition.ID, err)
		}
		leases = append(leases, lease)
	}
	for _, lease := range leases {
		lease.Release()
	}
}

func TestRegistryStartCloseStopsControllers(t *testing.T) {
	registry, err := NewRegistry([]*Instance{
		{Definition: Definition{ID: 1}, Auto: nil},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	registry.StartAll()
	registry.Close()
}
