// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeVirtualKeyboard,
		Desc:         "Checks Chrome virtual keyboard working on Android apps",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcKeyboardTest.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.BootedInTabletMode(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedInTabletMode(),
		}},
	})
}

func ChromeVirtualKeyboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
		cls = "org.chromium.arc.testapp.keyboard.MainActivity"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"
	)

	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Setting up app's initial state")
	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}
	if err := field.SetText(ctx, ""); err != nil {
		s.Fatal("Failed to empty field: ", err)
	}

	if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus a text field: ", err)
	}

	s.Log("Waiting for virtual keyboard to be ready")
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

	// Press a sequence of keys. Avoid using Space since it triggers autocomplete, which can
	// cause flaky failures: http://b/122456478#comment4
	keys := []string{
		"h", "e", "l", "l", "o", "w", "o",
		"backspace", "backspace", "t", "a", "s", "t"}

	expected := ""

	for _, key := range keys {
		if err := vkb.TapKey(ctx, kconn, key); err != nil {
			s.Fatalf("Failed to tap %q: %v", key, err)
		}

		if key == "backspace" {
			expected = expected[:len(expected)-1]
		} else {
			expected += key
		}

		// Check the input field after each keystroke to avoid flakiness. https://crbug.com/945729
		// In order to use GetText() after timeout, we should have shorter timeout than ctx.
		if err := d.Object(ui.ID(fieldID), ui.Text(expected)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := field.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field after typing %q", actual, expected)
			}
		}
	}
}
