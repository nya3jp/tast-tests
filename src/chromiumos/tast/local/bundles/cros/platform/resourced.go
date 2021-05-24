// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Resourced,
		Desc:         "Checks that resourced works",
		Contacts:     []string{"vovoy@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

const (
	dbusResourceManagerInterface = "org.chromium.ResourceManager"
	dbusResourceManagerPath      = "/org/chromium/ResourceManager"
	dbusResourceManagerService   = "org.chromium.ResourceManager"
)

func checkSetGameMode(ctx context.Context, s *testing.State) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusResourceManagerService, dbusResourceManagerInterface, dbusResourceManagerPath)
	if err != nil {
		s.Fatal("Unable to connect to Resource Manager: ", err)
	}

	// Get the original game mode.
	var origGameMode uint8
	if err = obj.Call(ctx, "GetGameMode").Store(&origGameMode); err != nil {
		s.Fatal("Unable to call method GetGameMode: ", err)
	}
	testing.ContextLog(ctx, "Original game mode: ", origGameMode)

	// Set game mode to different value.
	var newGameMode uint8
	if origGameMode == 0 {
		newGameMode = 1
	}
	if err = obj.Call(ctx, "SetGameMode", newGameMode).Err; err != nil {
		s.Fatal("Unable to call method SetGameMode: ", err)
	}
	testing.ContextLog(ctx, "Set game mode: ", newGameMode)

	// Check game mode is set to the new value.
	var gameMode uint8
	if err = obj.Call(ctx, "GetGameMode").Store(&gameMode); err != nil {
		s.Fatal("Unable to call method GetGameMode: ", err)
	}
	if newGameMode != gameMode {
		s.Error("Set game mode to: ", newGameMode, ", but got game mode: ", gameMode)
	}

	// Restore game mode.
	if err = obj.Call(ctx, "SetGameMode", origGameMode).Err; err != nil {
		s.Fatal("Unable to call method SetGameMode: ", err)
	}
}

func checkQueryMemoryStatus(ctx context.Context, s *testing.State) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusResourceManagerService, dbusResourceManagerInterface, dbusResourceManagerPath)
	if err != nil {
		s.Fatal("Unable to connect to Resource Manager: ", err)
	}

	var availableKB uint64
	if err = obj.Call(ctx, "GetAvailableMemoryKB").Store(&availableKB); err != nil {
		s.Fatal("Unable to call method GetAvailableMemoryKB: ", err)
	}
	testing.ContextLog(ctx, "GetAvailableMemoryKB returns: ", availableKB)

	var foregroundAvailableKB uint64
	if err = obj.Call(ctx, "GetForegroundAvailableMemoryKB").Store(&foregroundAvailableKB); err != nil {
		s.Fatal("Unable to call method GetForegroundAvailableMemoryKB: ", err)
	}
	testing.ContextLog(ctx, "GetForegroundAvailableMemoryKB returns: ", foregroundAvailableKB)

	var criticalMarginKB, moderateMarginKB uint64
	if err = obj.Call(ctx, "GetMemoryMarginsKB").Store(&criticalMarginKB, &moderateMarginKB); err != nil {
		s.Fatal("Unable to call method GetMemoryMarginsKB: ", err)
	}
	testing.ContextLog(ctx, "GetMemoryMarginsKB returns, critical: ", criticalMarginKB, ", moderate: ", moderateMarginKB)
}

func checkMemoryPressureSignal(ctx context.Context, s *testing.State) {
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
			s.Fatal("Wrong MemoryPressureChrome format")
		}
		testing.ContextLogf(ctx, "Got MemoryPressureChrome signal, level: %d, delta: %d", sig.Body[0], sig.Body[1])
	case <-ctx.Done():
		s.Fatal("Didn't get MemoryPressureChrome signal")
	}
}

func Resourced(ctx context.Context, s *testing.State) {
	checkSetGameMode(ctx, s)
	checkQueryMemoryStatus(ctx, s)

	ctxSignal, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	checkMemoryPressureSignal(ctxSignal, s)
}
