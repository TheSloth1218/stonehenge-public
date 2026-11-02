package testcontrol

import (
	"context"
	"testing"
	"time"
)

func TestPauseControlsRequireEnableAndToken(t *testing.T) {
	controls.mu.Lock()
	controls.enabled = false
	controls.token = "secret"
	controls.armed = make(map[string]*pauseState)
	controls.lastReleased = nil
	controls.mu.Unlock()

	if TokenOK("secret") {
		t.Fatal("token accepted while controls disabled")
	}
	if Arm(PauseRequest{Point: PointBeforeAutoSubmit}) {
		t.Fatal("arm succeeded while controls disabled")
	}

	controls.mu.Lock()
	controls.enabled = true
	controls.mu.Unlock()
	if !TokenOK("secret") {
		t.Fatal("token rejected while controls enabled")
	}
	if TokenOK("wrong") {
		t.Fatal("wrong token accepted")
	}
}

func TestPauseWaitAndRelease(t *testing.T) {
	controls.mu.Lock()
	controls.enabled = true
	controls.token = "secret"
	controls.armed = make(map[string]*pauseState)
	controls.lastReleased = nil
	controls.mu.Unlock()

	if !Arm(PauseRequest{Point: PointAfterCommit, TargetID: 42}) {
		t.Fatal("arm failed")
	}
	done := make(chan struct{})
	go func() {
		Wait(context.Background(), PointAfterCommit, 42, 7, 1)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		status := Status()
		if len(status.Paused) == 1 && status.Paused[0].AssignmentID == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pause status = %+v, want paused assignment", status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !Release(PointAfterCommit) {
		t.Fatal("release failed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait did not release")
	}
	if Status().LastReleased == nil {
		t.Fatal("last released pause not recorded")
	}
}
