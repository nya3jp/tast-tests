// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeVirtualKeyboard,
		Desc:         "Checks Chrome virtual keyboard working on Android apps",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"ArcKeyboardTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func injectTabletModeEvent(ctx context.Context, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	cmd := testexec.CommandContext(ctx, "inject_powerd_input_event", "--code=tablet", "--value="+value)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

func ChromeVirtualKeyboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
		cls = "org.chromium.arc.testapp.keyboard.MainActivity"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect power_manager: ", err)
	}

	sw, err := pm.GetSwitchStates(ctx)
	if err != nil {
		s.Fatal("Failed to get switch states: ", err)
	}

	if sw.TabletMode == nil {
		s.Fatal("tablet mode from power_manager unavailable")
	}

	if *sw.TabletMode == pmpb.SwitchStates_UNSUPPORTED {
		s.Log("Tablet switch unsupported on this device; skipping this test")
		return
	}

	if err := injectTabletModeEvent(ctx, true); err != nil {
		s.Fatal("Failed to set tablet mode: ", err)
	}
	defer injectTabletModeEvent(ctx, *sw.TabletMode == pmpb.SwitchStates_ON)

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		field := d.Object(ui.ID(fieldID))
		if err := field.WaitForExists(ctx); err != nil {
			s.Fatal("Failed to find field: ", err)
		}
		if err := field.Click(ctx); err != nil {
			s.Fatal("Failed to click field: ", err)
		}
		if err := field.SetText(ctx, ""); err != nil {
			s.Fatal("Failed to empty field: ", err)
		}

		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
		}
		if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
		}

		kconn, err := vkb.UIConn(ctx, cr)
		if err != nil {
			s.Fatal("Creating connection to virtual keyboard UI failed: ", err)
		}
		defer kconn.Close()

		// Press a sequence of keys.
		keys := []string{
			"h", "e", "l", "l", "o", "space", "w", "o",
			"backspace", "backspace", "t", "a", "s", "t"}

		for _, key := range keys {
			if err := vkb.TapKey(ctx, kconn, key); err != nil {
				s.Fatalf("Failed to tap %q: %v", key, err)
			}
		}

		const expected = "hello tast"

		if actual, err := field.GetText(ctx); err != nil {
			s.Fatal("Failed to get text: ", err)
		} else if actual != expected {
			s.Errorf("Got input %q from field after typing %q", actual, expected)
		}
	})
}
