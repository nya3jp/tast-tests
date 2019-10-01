// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SoftInputMode,
		Desc:         "Checks that softInputMode is properly handled",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcSoftInputModeTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func SoftInputMode(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	settingsAct, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer settingsAct.Close()

	if err := settingsAct.Start(ctx); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	if err := settingsAct.WaitForIdle(ctx, 30*time.Second); err != nil {
	}

	const (
		apk = "ArcSoftInputModeTest.apk"
		pkg = "org.chromium.arc.testapp.softinputmode"
		cls = ".AdjustPanActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	if err := act.WaitForIdle(ctx, 30*time.Second); err != nil {
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventSnapLeft); err != nil {
		s.Fatal("Failed to snap app in split view: ", err)
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, settingsAct.PackageName(), ash.WMEventSnapRight); err != nil {
		s.Fatal("Failed to snap app in split view: ", err)
	}

	const fieldID = "org.chromium.arc.testapp.softinputmode:id/text"
	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	if err := d.WaitForIdle(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}
}
