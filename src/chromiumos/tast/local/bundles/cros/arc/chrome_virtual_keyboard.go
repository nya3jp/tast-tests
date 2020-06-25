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

// vkTestFunc is a signature of a "test" function.
type vkTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *chrome.Chrome, *ui.Device, *testing.State)

// vkTestParams represents the name of the test and the function to call.
type vkTestParams struct {
	name string
	fn   vkTestFunc
}

var stableVkTests = []vkTestParams{
	{"Basic editing", chromeVirtualKeyboardBasicEditingTest},
	{"Focus change", chromeVirtualKeyboardFocusChangeTest},
}

var unstableVkTests = []vkTestParams{
	// TODO(b/157432003) Stabilize this test.
	{"Editing on TYPE_NULL", chromeVirtualKeyboardEditingOnNullTypeTest},
}

const virtualKeyboardTestAppPkg = "org.chromium.arc.testapp.keyboard"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeVirtualKeyboard,
		Desc:         "Checks Chrome virtual keyboard working on Android apps",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.BootedInTabletMode(),
		Params: []testing.Param{{
			Val:               stableVkTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               stableVkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "unstable",
			Val:               unstableVkTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "unstable_vm",
			Val:               unstableVkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
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
	defer act.Stop(ctx, tconn)

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

	// Press a sequence of keys. Avoid using Space since it triggers autocomplete, which can
	// cause flaky failures: http://b/122456478#comment4
	keys := []string{
		"h", "e", "l", "l", "o", "w", "o",
		"backspace", "backspace", "t", "a", "s", "t"}

	expected := ""

	for _, key := range keys {
		if err := vkb.TapKey(ctx, tconn, key); err != nil {
			s.Fatalf("Failed to tap %q: %v", key, err)
		}

		if key == "backspace" {
			expected = expected[:len(expected)-1]
		} else {
			expected += key
		}

		// Check the input field after each keystroke to avoid flakiness. https://crbug.com/945729
		// In order to use GetText() after timeout, we should have shorter timeout than ctx.
		if err := field.WaitForText(ctx, expected, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for text: ", err)
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
	defer act.Stop(ctx, tconn)

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
// TODO(crbug.com/1081596): Add tests with an IME with composition.
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
	defer act.Stop(ctx, tconn)

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
		if err := vkb.TapKey(ctx, tconn, key.Key); err != nil {
			s.Fatalf("Failed to tap %q: %v", key.Key, err)
		}

		// Check the input field after each keystroke.
		expectedText := fmt.Sprintf("key down: keyCode=%d", key.Expected)
		if err := keyDownLabel.WaitForText(ctx, expectedText, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for text: ", err)
		}
		expectedText = fmt.Sprintf("key up: keyCode=%d", key.Expected)
		if err := keyUpLabel.WaitForText(ctx, expectedText, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for text: ", err)
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

	for _, test := range s.Param().([]vkTestParams) {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			test.fn(ctx, tconn, a, cr, d, s)
		})
	}
}
