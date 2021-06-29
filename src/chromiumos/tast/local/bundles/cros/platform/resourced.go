// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

type resourcedTestParams struct {
	isBaseline bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Resourced,
		Desc:         "Checks that resourced works",
		Contacts:     []string{"vovoy@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: resourcedTestParams{
				isBaseline: false,
			},
		}, {
			Name:      "baseline",
			ExtraAttr: []string{"group:mainline"},
			Val: resourcedTestParams{
				isBaseline: true,
			},
		}},
	})
}

const (
	dbusResourceManagerInterface = "org.chromium.ResourceManager"
	dbusResourceManagerPath      = "/org/chromium/ResourceManager"
	dbusResourceManagerService   = "org.chromium.ResourceManager"
)

func checkSetGameMode(ctx context.Context) error {
	obj, err := dbusutil.NewDBusObject(ctx, dbusResourceManagerService, dbusResourceManagerInterface, dbusResourceManagerPath)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Resource Manager")
	}

	// Get the original game mode.
	var origGameMode uint8
	if err = obj.Call(ctx, "GetGameMode").Store(&origGameMode); err != nil {
		return errors.Wrap(err, "failed to call method GetGameMode")
	}
	testing.ContextLog(ctx, "Original game mode: ", origGameMode)

	// Set game mode to different value.
	var newGameMode uint8
	if origGameMode == 0 {
		newGameMode = 1
	}
	if err = obj.Call(ctx, "SetGameMode", newGameMode).Err; err != nil {
		return errors.Wrap(err, "failed to call method SetGameMode")
	}
	testing.ContextLog(ctx, "Set game mode: ", newGameMode)

	// Check game mode is set to the new value.
	var gameMode uint8
	if err = obj.Call(ctx, "GetGameMode").Store(&gameMode); err != nil {
		return errors.Wrap(err, "failed to call method GetGameMode")
	}
	if newGameMode != gameMode {
		return errors.Errorf("set game mode to: %d, but got game mode: %d", newGameMode, gameMode)
	}

	// Restore game mode.
	if err = obj.Call(ctx, "SetGameMode", origGameMode).Err; err != nil {
		return errors.Wrap(err, "failed to call method SetGameMode")
	}
	return nil
}

func checkQueryMemoryStatus(ctx context.Context) error {
	obj, err := dbusutil.NewDBusObject(ctx, dbusResourceManagerService, dbusResourceManagerInterface, dbusResourceManagerPath)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Resource Manager")
	}

	var availableKB uint64
	if err = obj.Call(ctx, "GetAvailableMemoryKB").Store(&availableKB); err != nil {
		return errors.Wrap(err, "failed to call method GetAvailableMemoryKB")
	}
	testing.ContextLog(ctx, "GetAvailableMemoryKB returns: ", availableKB)

	var foregroundAvailableKB uint64
	if err = obj.Call(ctx, "GetForegroundAvailableMemoryKB").Store(&foregroundAvailableKB); err != nil {
		return errors.Wrap(err, "failed to call method GetForegroundAvailableMemoryKB")
	}
	testing.ContextLog(ctx, "GetForegroundAvailableMemoryKB returns: ", foregroundAvailableKB)

	var criticalMarginKB, moderateMarginKB uint64
	if err = obj.Call(ctx, "GetMemoryMarginsKB").Store(&criticalMarginKB, &moderateMarginKB); err != nil {
		return errors.Wrap(err, "failed to call method GetMemoryMarginsKB")
	}
	testing.ContextLog(ctx, "GetMemoryMarginsKB returns, critical: ", criticalMarginKB, ", moderate: ", moderateMarginKB)

	return nil
}

func checkMemoryPressureSignal(ctx context.Context) error {
	// Check MemoryPressureChrome signal is sent.
	pressure, _ := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusResourceManagerPath,
		Interface: dbusResourceManagerInterface,
		Member:    "MemoryPressureChrome",
	})
	defer pressure.Close(ctx)

	select {
	case sig := <-pressure.Signals:
		if len(sig.Body) != 2 {
			return errors.New("wrong MemoryPressureChrome format")
		}
		testing.ContextLogf(ctx, "Got MemoryPressureChrome signal, level: %d, delta: %d", sig.Body[0], sig.Body[1])
	case <-ctx.Done():
		return errors.New("didn't get MemoryPressureChrome signal")
	}

	return nil
}

func Resourced(ctx context.Context, s *testing.State) {
	// Baseline checks.
	if err := checkSetGameMode(ctx); err != nil {
		s.Fatal("Checking SetGameMode failed: ", err)
	}

	if s.Param().(resourcedTestParams).isBaseline {
		return
	}

	// Other checks.
	if err := checkQueryMemoryStatus(ctx); err != nil {
		s.Fatal("Querying memory status failed: ", err)
	}

	ctxSignal, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := checkMemoryPressureSignal(ctxSignal); err != nil {
		s.Fatal("Checking memory pressure signal failed: ", err)
	}
}
