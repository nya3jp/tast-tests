// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Close,
		Desc:         "Checks that no way to close a kiosk app manually",
		LacrosStatus: testing.LacrosVariantExists,
		Contacts: []string{
			"pbond@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.KioskLoggedInLacros,
	})
}

func Close(ctx context.Context, s *testing.State) {
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	testing.ContextLog(ctx, "Waiting for splash screen is gone")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return kioskmode.IsKioskAppStarted(ctx)
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Splash screen is not gone: ", err)
	}
	testing.Sleep(ctx, time.Minute)
	for _, keySequence := range []string{"Ctrl+Alt+Q", "Ctrl+Alt+W"} {
		if err := kw.Accel(ctx, keySequence); err != nil {
			s.Fatalf("Failed to hit %s and attempt to quit a kiosk app: %v", keySequence, err)
		}
		if err := kioskmode.IsKioskAppStarted(ctx); err != nil {
			s.Fatalf("%s quit a kiosk app: %v", keySequence, err)
		}
	}
}
