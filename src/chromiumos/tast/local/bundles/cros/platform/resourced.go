// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/resourced"
	"chromiumos/tast/testing"
)

type resourcedTestParams struct {
	isBaseline bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Resourced,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that resourced works",
		Contacts:     []string{"vovoy@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraAttr: []string{"informational"},
			Val: resourcedTestParams{
				isBaseline: false,
			},
		}, {
			Name: "baseline",
			Val: resourcedTestParams{
				isBaseline: true,
			},
		}},
	})
}

func checkSetGameMode(ctx context.Context, rm *resourced.Client) (resErr error) {
	// Get the original game mode.
	origGameMode, err := rm.GameMode(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query game mode state")
	}
	testing.ContextLog(ctx, "Original game mode: ", origGameMode)

	defer func() {
		// Restore game mode.
		if err = rm.SetGameMode(ctx, origGameMode); err != nil {
			if resErr == nil {
				resErr = errors.Wrap(err, "failed to reset game mode state")
			} else {
				testing.ContextLog(ctx, "Failed to reset game mode state: ", err)
			}
		}
	}()

	// Set game mode to different value.
	var newGameMode uint8
	if origGameMode == 0 {
		newGameMode = 1
	}
	if err = rm.SetGameMode(ctx, newGameMode); err != nil {
		return errors.Wrap(err, "failed to set game mode state")
	}
	testing.ContextLog(ctx, "Set game mode: ", newGameMode)

	// Check game mode is set to the new value.
	gameMode, err := rm.GameMode(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query game mode state")
	}
	if newGameMode != gameMode {
		return errors.Errorf("set game mode to: %d, but got game mode: %d", newGameMode, gameMode)
	}
	return nil
}

func checkSetGameModeWithTimeout(ctx context.Context, rm *resourced.Client) (resErr error) {
	var newGameMode uint8 = resourced.GameModeBorealis
	if err := rm.SetGameModeWithTimeout(ctx, newGameMode, 1); err != nil {
		return errors.Wrap(err, "failed to set game mode state")
	}
	testing.ContextLog(ctx, "Set game mode with 1 second timeout: ", newGameMode)

	// Check game mode is set to the new value.
	gameMode, err := rm.GameMode(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query game mode state")
	}
	if newGameMode != gameMode {
		return errors.Errorf("set game mode to: %d, but got game mode: %d", newGameMode, gameMode)
	}

	// Check game mode is reset after timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		gameMode, err := rm.GameMode(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to query game mode state")
		}
		if gameMode != resourced.GameModeOff {
			return errors.New("game mode is not reset")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to wait for game mode reset")
	}

	return nil
}

func checkQueryMemoryStatus(ctx context.Context, rm *resourced.Client) error {
	availableKB, err := rm.AvailableMemoryKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query available memory")
	}
	testing.ContextLog(ctx, "GetAvailableMemoryKB returns: ", availableKB)

	foregroundAvailableKB, err := rm.ForegroundAvailableMemoryKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query foreground available memory")
	}
	testing.ContextLog(ctx, "GetForegroundAvailableMemoryKB returns: ", foregroundAvailableKB)

	m, err := rm.MemoryMarginsKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query memory margins")
	}
	testing.ContextLog(ctx, "GetMemoryMarginsKB returns, critical: ", m.CriticalKB, ", moderate: ", m.ModerateKB)

	componentMemoryMargins, err := rm.ComponentMemoryMarginsKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query component memory margins")
	}

	testing.ContextLogf(ctx, "GetComponentMemoryMarginsKB returns %+v", componentMemoryMargins)

	return nil
}

func checkMemoryPressureSignal(ctx context.Context, rm *resourced.Client) error {
	// Check MemoryPressureChrome signal is sent.
	ctxWatcher, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	watcher, err := rm.NewChromePressureWatcher(ctxWatcher)
	if err != nil {
		return errors.Wrap(err, "failed to create PressureWatcher")
	}
	defer watcher.Close(ctx)

	select {
	case sig := <-watcher.Signals:
		testing.ContextLogf(ctx, "Got MemoryPressureChrome signal, level: %d, delta: %d", sig.Level, sig.Delta)
	case <-ctxWatcher.Done():
		return errors.New("didn't get MemoryPressureChrome signal")
	}

	return nil
}

func checkSetRTCAudioActive(ctx context.Context, rm *resourced.Client) error {
	// Get the original RTC audio active.
	origRTCAudioActive, err := rm.RTCAudioActive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query RTC audio active")
	}
	testing.ContextLog(ctx, "Original RTC audio active: ", origRTCAudioActive)

	defer func() {
		// Restore RTC audio active.
		if err = rm.SetRTCAudioActive(ctx, origRTCAudioActive); err != nil {
			testing.ContextLog(ctx, "Failed to reset RTC audio active: ", err)
		}
	}()

	// Set RTC audio ative to different value.
	newRTCAudioActive := resourced.RTCAudioActiveOff
	if origRTCAudioActive == resourced.RTCAudioActiveOff {
		newRTCAudioActive = resourced.RTCAudioActiveOn
	}
	if err = rm.SetRTCAudioActive(ctx, newRTCAudioActive); err != nil {
		// On machines not supporting Intel hardware EPP, SetRTCAudioActive returning error is expected.
		testing.ContextLog(ctx, "Failed to set RTC audio active: ", err)
	}
	testing.ContextLog(ctx, "Set RTC audio active: ", newRTCAudioActive)

	// Check RTC audio active is set to the new value.
	rtcAudioActive, err := rm.RTCAudioActive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query RTC audio active")
	}
	if newRTCAudioActive != rtcAudioActive {
		return errors.Errorf("failed to set RTC audio active: got %d, want: %d", rtcAudioActive, newRTCAudioActive)
	}
	return nil
}

func checkSetFullscreenVideo(ctx context.Context, rm *resourced.Client) (resErr error) {
	var newFullscreenVideo uint8 = resourced.FullscreenVideoActive
	var timeout uint32 = 1
	if err := rm.SetFullscreenVideoWithTimeout(ctx, newFullscreenVideo, timeout); err != nil {
		// On machines not supporting Intel hardware EPP, SetFullscreenVideoWithTimeout returning error is expected.
		testing.ContextLog(ctx, "Failed to set full screen video active: ", err)
		return nil
	}
	testing.ContextLogf(ctx, "Set full screen video active to %d with %d second timeout", newFullscreenVideo, timeout)

	// Check full screen video state is set to the new value.
	fullscreenVideo, err := rm.FullscreenVideo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query full screen video state")
	}
	if newFullscreenVideo != fullscreenVideo {
		return errors.Errorf("failed to set full screen video state: got %d, want: %d", fullscreenVideo, newFullscreenVideo)
	}

	// Check full screen video state is reset after timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		fullscreenVideo, err := rm.FullscreenVideo(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to query full screen video state")
		}
		if fullscreenVideo != resourced.FullscreenVideoInactive {
			return errors.New("full screen video state is not reset")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Duration(2*timeout) * time.Second, Interval: 100 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to wait for full screen video state reset")
	}

	return nil
}

func checkPowerSupplyChange(ctx context.Context, rm *resourced.Client) (resErr error) {
	// Check PowerSupplyChange method can be called successfully.
	if err := rm.PowerSupplyChange(ctx); err != nil {
		return errors.Wrap(err, "failed to call power supply change")
	}

	return nil
}

func Resourced(ctx context.Context, s *testing.State) {
	rm, err := resourced.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to create Resource Manager client: ", err)
	}

	if s.Param().(resourcedTestParams).isBaseline {
		// Baseline checks.
		if err := checkSetGameMode(ctx, rm); err != nil {
			s.Fatal("Checking SetGameMode failed: ", err)
		}

		if err := checkQueryMemoryStatus(ctx, rm); err != nil {
			s.Fatal("Querying memory status failed: ", err)
		}

		if err := checkMemoryPressureSignal(ctx, rm); err != nil {
			s.Fatal("Checking memory pressure signal failed: ", err)
		}

		if err := checkSetGameModeWithTimeout(ctx, rm); err != nil {
			s.Fatal("Checking SetGameModeWithTimeout failed: ", err)
		}

		if err := checkSetRTCAudioActive(ctx, rm); err != nil {
			s.Fatal("Checking SetRTCAudioActive failed: ", err)
		}

		if err := checkSetFullscreenVideo(ctx, rm); err != nil {
			s.Fatal("Checking SetFullscreenVideoWithTimeout failed: ", err)
		}

		if err := checkPowerSupplyChange(ctx, rm); err != nil {
			s.Fatal("Checking PowerSupplyChange failed: ", err)
		}

		return
	}

	// New tests will be added here. Stable tests are promoted to baseline.
}
