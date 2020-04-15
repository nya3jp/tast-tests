// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
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
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedInTabletMode(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedInTabletMode(),
		}},
	})
}

// chromeVirtualKeyboardBasicEditingTest tests basic editing on a EditText on an ARC app by Chrome's virtual keyboard.
func chromeVirtualKeyboardBasicEditingTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".MainActivity"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx)

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

// chromeVirtualKeyboardFocusChangeTest tests the virtual keyboard behavior when the focus moves programmatically.
func chromeVirtualKeyboardFocusChangeTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".FocusChangeTestActivity"

		buttonID1 = pkg + ":id/focus_switch_button"
		buttonID2 = pkg + ":id/hide_and_focus_switch_button"
		buttonID3 = pkg + ":id/hide_button"
		fieldID1  = pkg + ":id/text1"
		fieldID2  = pkg + ":id/text2"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx)

	// Make sure that the virtual keyboard is hidden now. It is the precondition of this test.
	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to request to hide the virtual keyboard: ", err)
	}
	if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
		s.Fatal("Failed to hide the virtual keyboard: ", err)
	}

	// Focusing on the text field programmatically should not show the virtual keyboard.
	button := d.Object(ui.ID(buttonID1))
	if err := button.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the button: ", err)
	}
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := d.Object(ui.ID(fieldID1), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Pressing the button didn't cause focusing on the field: ", err)
	}
	shown, err := vkb.IsShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the virtual keyboard visibility: ", err)
	}
	if shown {
		s.Fatal("The virtual keyboard is shown without any user action")
	}

	// Clicking on the text field should show the virtual keyboard.
	field1 := d.Object(ui.ID(fieldID1))
	if err := field1.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the field: ", err)
	}
	if err := field1.Click(ctx); err != nil {
		s.Fatal("Failed to click the field: ", err)
	}

	s.Log("Waiting for the virtual keyboard to be ready")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	// The virtual keyboard should keep showing when the focus is moved between the text fields programmatically.
	s.Log("Clicking the button to switch the focus")
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := d.Object(ui.ID(fieldID2), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Clicking the button didn't cause the focus move: ", err)
	}
	shown, err = vkb.IsShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the virtual keyboard visibility: ", err)
	}
	if !shown {
		s.Fatal("The focus move makes the virtual keyboard to be hidden")
	}

	s.Log("Clicking the button to hide the virtual keyboard and switch the focus")
	button2 := d.Object(ui.ID(buttonID2))
	if err := button2.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := d.Object(ui.ID(fieldID1), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Clicking the button didn't cause the focus move: ", err)
	}
	if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
		s.Fatal("The virtual keyboard doesn't hide")
	}

	// Make sure that hideSoftInputFromWindow() works.
	if err := field1.Click(ctx); err != nil {
		s.Fatal("Failed to click the field: ", err)
	}
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}
	s.Log("Clicking the button to hide the virtual keyboard")
	button3 := d.Object(ui.ID(buttonID3))
	if err := button3.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
		s.Fatal("Failed to hide the virtual keyboard: ", err)
	}
}

func chromeVirtualKeyboardRotationTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".MainActivity"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx)

	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
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

	// Chrome OS virtual keyboard is shown and ready. Let's rotate the device.
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) == 0 {
		s.Fatal("No display found")
	}
	var info *display.Info
	for i := range infos {
		if infos[i].IsInternal {
			info = &infos[i]
		}
	}
	if info == nil {
		s.Log("No internal display found. Default to the first display")
		info = &infos[0]
	}

	waitForRotation := func(expectLandscape bool) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer disp.Close()
			s, err := disp.Size(ctx)
			if err != nil {
				// It may return error while transition, keep retrying.
				return err
			}
			if s.Width > s.Height == expectLandscape {
				return nil
			}

			return errors.New("display not rotated in ARC")
		}, nil)
	}

	// Restore the initial rotation after the test.
	defer func() {
		if err := display.SetDisplayProperties(ctx, tconn, info.ID,
			display.DisplayProperties{Rotation: &info.Rotation}); err != nil {
			s.Fatal("Failed to restore the initial rotation: ", err)
		}
	}()

	// Try all rotations
	rotations := []int{0, 90, 180, 270}
	for _, r := range rotations {
		if err := display.SetDisplayProperties(ctx, tconn, info.ID,
			display.DisplayProperties{Rotation: &r}); err != nil {
			s.Fatalf("Failed to rotate display to %d: %q", r, err)
		}
		if err := waitForRotation((r % 180) == 0); err != nil {
			s.Fatal("Failed to wait for rotation: ", err)
		}
		testing.Sleep(ctx, 3*time.Second)
		if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
			s.Fatalf("Failed to wait for the virtual keyboard to show at rotation %d: %q", r, err)
		}

	}
}

func ChromeVirtualKeyboard(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	a := p.ARC
	cr := p.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const apk = "ArcKeyboardTest.apk"
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Run(ctx, "editing", func(ctx context.Context, s *testing.State) {
		chromeVirtualKeyboardBasicEditingTest(ctx, tconn, a, cr, d, s)
	})
	s.Run(ctx, "focusChange", func(ctx context.Context, s *testing.State) {
		chromeVirtualKeyboardFocusChangeTest(ctx, tconn, a, cr, d, s)
	})
	s.Run(ctx, "rotation", func(ctx context.Context, s *testing.State) {
		chromeVirtualKeyboardRotationTest(ctx, tconn, a, cr, d, s)
	})
}
