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
		Timeout:      2 * time.Minute,
	})
}

func Resourced(ctx context.Context, s *testing.State) {
	// Check MemoryPressureChrome signal is sent.
	pressure, _ := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      "/org/chromium/ResourceManager",
		Interface: "org.chromium.ResourceManager",
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
