// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GlobalActionsMenu,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if showing and hiding global actions work on ARC",
		Contacts:     []string{"nergi@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline"},
		Fixture:      "arcBooted",
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      3 * time.Minute,
	})
}

func GlobalActionsMenu(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	ui := uiauto.New(tconn)

	// Open global action menu
	if err := a.Command(ctx, "input", "keyevent", "--longpress", "KEYCODE_POWER").Run(); err != nil {
		s.Fatal("Failed to launch global actions menu via ADB command: ", err)
	}

	powerButtonMenu := nodewith.ClassName("PowerButtonMenuScreenView")
	if err := ui.WaitUntilExists(powerButtonMenu)(ctx); err != nil {
		s.Fatal("Failed to find PowerButtonMenuScreenView: ", err)
	}

	// Close global action menu
	if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.CLOSE_SYSTEM_DIALOGS").Run(); err != nil {
		s.Fatal("Failed to close global actions menu via ADB command: ", err)
	}

	if err := ui.WaitUntilGone(powerButtonMenu)(ctx); err != nil {
		s.Fatal("PowerButtonMenuScreenView is not dismissed: ", err)
	}
}
