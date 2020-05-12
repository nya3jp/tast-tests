// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

const virtualKeyboardTestAppPkg = "org.chromium.arc.testapp.keyboard"

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
		activityName = ".MainActivity"

		fieldID = virtualKeyboardTestAppPkg + ":id/text"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, virtualKeyboardTestAppPkg, activityName)
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
		activityName = ".FocusChangeTestActivity"

		buttonID1 = virtualKeyboardTestAppPkg + ":id/focus_switch_button"
		buttonID2 = virtualKeyboardTestAppPkg + ":id/hide_and_focus_switch_button"
		buttonID3 = virtualKeyboardTestAppPkg + ":id/hide_button"
		fieldID1  = virtualKeyboardTestAppPkg + ":id/text1"
		fieldID2  = virtualKeyboardTestAppPkg + ":id/text2"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, virtualKeyboardTestAppPkg, activityName)
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

// chromeVirtualKeyboardEditingOnNullTypeTest tests the virtual keyboard behavior on an EditText with InputType.TYPE_NULL
// The virtual keyboard should send a key event instead of inserting text through InputConnection on such an EditText.
func chromeVirtualKeyboardEditingOnNullTypeTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		activityName = ".NullEditTextActivity"

		editTextID         = virtualKeyboardTestAppPkg + ":id/text"
		lastKeyDownLabelID = virtualKeyboardTestAppPkg + ":id/last_key_down"
		lastKeyUpLabelID   = virtualKeyboardTestAppPkg + ":id/last_key_up"
	)
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	act, err := arc.NewActivity(a, virtualKeyboardTestAppPkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx)

	editText := d.Object(ui.ID(editTextID))
	if err := editText.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := editText.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}
	if err := editText.SetText(ctx, ""); err != nil {
		s.Log("Failed to empty field: ", err)
	}

	if err := d.Object(ui.ID(editTextID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
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

	keyDownLabel := d.Object(ui.ID(lastKeyDownLabelID))
	keyUpLabel := d.Object(ui.ID(lastKeyUpLabelID))
	for _, key := range []struct {
		Key      string
		Expected int
	}{
		{"a", 29},         // AKEYCODE_A
		{"b", 30},         // AKEYCODE_B
		{"c", 31},         // AKEYCODE_C
		{"backspace", 67}, // AKEYCODE_DEL
		{"enter", 66},     // AKEYCODE_ENTER
	} {
		if err := vkb.TapKey(ctx, kconn, key.Key); err != nil {
			s.Fatalf("Failed to tap %q: %v", key.Key, err)
		}

		// Check the input field after each keystroke.
		expectedText := fmt.Sprintf("key down: keyCode=%d", key.Expected)
		if err := d.Object(ui.ID(lastKeyDownLabelID), ui.Text(expectedText)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := keyDownLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, expectedText)
			}
		}
		expectedText = fmt.Sprintf("key up: keyCode=%d", key.Expected)
		if err := d.Object(ui.ID(lastKeyUpLabelID), ui.Text(expectedText)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := keyUpLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, expectedText)
			}
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
	// TODO(crbug.com/1081596): Add tests with an IME with composition.
	s.Run(ctx, "editingOnNull", func(ctx context.Context, s *testing.State) {
		chromeVirtualKeyboardEditingOnNullTypeTest(ctx, tconn, a, cr, d, s)
	})
}
