// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/coords"
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
	{"Editing on TYPE_NULL", chromeVirtualKeyboardEditingOnNullTypeTest},
	{"Number input", chromeVirtualKeyboardNumberInputTest},
	{"Password editing", chromeVirtualKeyboardPasswordEditingTest},
}

var unstableVkTests = []vkTestParams{
	{"Focus change", chromeVirtualKeyboardFocusChangeTest},
	{"Floating mode", chromeVirtualKeyboardFloatingTest},
	{"Rotation", chromeVirtualKeyboardRotationTest},
}

const virtualKeyboardTestAppPkg = "org.chromium.arc.testapp.keyboard"

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeVirtualKeyboard, LacrosStatus: testing.LacrosVariantUnknown, Desc: "Checks Chrome virtual keyboard working on Android apps",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedInTabletMode",
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
			Timeout:           8 * time.Minute,
		}, {
			Name:              "unstable_vm",
			Val:               unstableVkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           8 * time.Minute,
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

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(ctx)

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
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
	}

	// Press a sequence of keys. Avoid using Space since it triggers autocomplete, which can
	// cause flaky failures: http://b/122456478#comment4
	keys := []string{
		"h", "e", "l", "l", "o", "w", "o",
		"backspace", "backspace", "t", "a", "s", "t"}

	expected := ""

	for _, key := range keys {
		if err := vkbCtx.TapKey(key)(ctx); err != nil {
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

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(ctx)

	act, err := arc.NewActivity(a, virtualKeyboardTestAppPkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	// Make sure that the focus is on the first field.
	// Clicking on the text field should show the virtual keyboard.
	field1 := d.Object(ui.ID(fieldID1))
	if err := field1.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the field: ", err)
	}
	if err := field1.Click(ctx); err != nil {
		s.Fatal("Failed to click the field: ", err)
	}
	if err := d.Object(ui.ID(fieldID1), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus the field: ", err)
	}
	s.Log("Waiting for the virtual keyboard to be ready")
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	// The virtual keyboard should keep showing when the focus is moved between the text fields programmatically.
	s.Log("Clicking the button to switch the focus")
	focusSwitchButton := d.Object(ui.ID(buttonID1))
	if err := focusSwitchButton.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the button: ", err)
	}
	if err := focusSwitchButton.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := d.Object(ui.ID(fieldID2), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Clicking the button didn't cause the focus move: ", err)
	}
	shown, err := vkbCtx.IsShown(ctx)
	if err != nil {
		s.Fatal("Failed to get the virtual keyboard visibility: ", err)
	}
	if !shown {
		s.Fatal("The focus move makes the virtual keyboard to be hidden")
	}

	// Hide the virtual keyboard.
	if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to request to hide the virtual keyboard: ", err)
	}

	// Moving focus to the other text field programmatically should not show the virtual keyboard.
	if err := focusSwitchButton.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := d.Object(ui.ID(fieldID1), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Pressing the button didn't cause focusing on the field: ", err)
	}
	shown, err = vkbCtx.IsShown(ctx)
	if err != nil {
		s.Fatal("Failed to get the virtual keyboard visibility: ", err)
	}
	if shown {
		s.Fatal("The virtual keyboard is shown without any user action")
	}

	// Make sure that hideSoftInputFromWindow() works.
	if err := field1.Click(ctx); err != nil {
		s.Fatal("Failed to click the field: ", err)
	}
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Clicking the button to hide the virtual keyboard")
	button3 := d.Object(ui.ID(buttonID3))
	if err := button3.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	if err := vkbCtx.WaitUntilHidden()(ctx); err != nil {
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

	vkbCtx := vkb.NewContext(cr, tconn)

	defer vkbCtx.HideVirtualKeyboard()(ctx)

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

	// No need to wait for decoder enabled because the decoder won't be enabled on TYPE_NULL field.
	s.Log("Waiting for virtual keyboard to be ready")
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
	}

	keyDownLabel := d.Object(ui.ID(lastKeyDownLabelID))
	keyUpLabel := d.Object(ui.ID(lastKeyUpLabelID))
	for _, key := range []struct {
		Key      string
		Expected int
	}{
		{"0", 7},          // AKEYCODE_0
		{"7", 14},         // AKEYCODE_7
		{"a", 29},         // AKEYCODE_A
		{"b", 30},         // AKEYCODE_B
		{"c", 31},         // AKEYCODE_C
		{"backspace", 67}, // AKEYCODE_DEL
		{"enter", 66},     // AKEYCODE_ENTER
	} {
		if err := vkbCtx.TapKey(key.Key)(ctx); err != nil {
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

func chromeVirtualKeyboardFloatingTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		activityName = ".OverscrollTestActivity"
		fieldID      = virtualKeyboardTestAppPkg + ":id/text"
	)
	vkbCtx := vkb.NewContext(cr, tconn)

	defer vkbCtx.HideVirtualKeyboard()(ctx)

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

	initialBounds, err := field.GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the bounds of the field: ", err)
	}
	s.Logf("The initial bounds of the field is %q", initialBounds)

	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	s.Log("Waiting for the virtual keyboard to be ready")
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
	}

	// Showing the normal virtual keyboard should push up the view.
	boundsWithVK, err := field.GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the bounds of the field: ", err)
	}
	if initialBounds.CenterPoint().Y <= boundsWithVK.CenterPoint().Y {
		s.Fatalf("VK doesn't push up the focused view: %q -> %q", initialBounds, boundsWithVK)
	}
	s.Logf("The bounds with the normal VK is %q", boundsWithVK)

	waitForRelayout := func(expected coords.Rect) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			bounds, err := field.GetBounds(ctx)
			if err != nil {
				return testing.PollBreak(err)
			}
			if expected != bounds {
				return errors.Errorf("the field doesn't move: %q != %q", expected, bounds)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}

	// Switching the VK to floating mode.
	if err := vkbCtx.SetFloatingMode(true)(ctx); err != nil {
		s.Fatal("Failed to switch to floating mode: ", err)
	}
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}
	if err := waitForRelayout(initialBounds); err != nil {
		s.Fatal("Failed to move back the field by switching to floating mode: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	// Switching back to the normal mode
	if err := vkbCtx.SetFloatingMode(false)(ctx); err != nil {
		s.Fatal("Failed to switch to dock mode: ", err)
	}
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}
	if err := waitForRelayout(boundsWithVK); err != nil {
		s.Fatal("Failed to move up the field by switching to normal mode: ", err)
	}
}

func chromeVirtualKeyboardRotationTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		activityName = ".MainActivity"

		fieldID = virtualKeyboardTestAppPkg + ":id/text"
	)

	vkbCtx := vkb.NewContext(cr, tconn)

	defer vkbCtx.HideVirtualKeyboard()(ctx)

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
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
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

	// Make sure to have 0 degrees rotation before testing.
	if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0); err != nil {
		s.Fatal("Failed to set 0 degrees rotation: ", err)
	}

	// Try all rotations
	for _, r := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {

		coordsBefore, err := vkbCtx.Location(ctx)
		if err != nil {
			s.Fatal("Failed to get the virtual keyboard location: ", err)
		}

		if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, r); err != nil {
			s.Fatalf("Failed to rotate display to %q: %q", r, err)
		}

		coordsAfter, err := vkbCtx.Location(ctx)
		if err != nil {
			s.Fatal("Failed to get the virtual keyboard location after display rotation: ", err)
		}

		if coordsBefore == coordsAfter || coordsAfter.Empty() {
			s.Fatalf("Failed to show the virtual keyboard after rotation in %s, before %q; after %q", r, coordsBefore, coordsAfter)
		}
	}
}

// chromeVirtualKeyboardPasswordEditingTest tests editing on a password field on an ARC app by Chrome's virtual keyboard.
func chromeVirtualKeyboardPasswordEditingTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		activityName = ".MainActivity"

		passwordFieldID = virtualKeyboardTestAppPkg + ":id/password"
	)

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(ctx)

	act, err := arc.NewActivity(a, virtualKeyboardTestAppPkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	field := d.Object(ui.ID(passwordFieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}
	if err := field.SetText(ctx, ""); err != nil {
		s.Fatal("Failed to empty field: ", err)
	}

	if err := d.Object(ui.ID(passwordFieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus a text field: ", err)
	}

	s.Log("Waiting for virtual keyboard to be ready")
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}
	// We should not wait for the decoder because it is not enabled on the password field.

	// Press a sequence of keys.
	keys := []string{"p", "a", "s", "s", "w", "o", "r", "d", "backspace", "backspace"}

	expected := ""

	for _, key := range keys {
		if err := vkbCtx.TapKey(key)(ctx); err != nil {
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
			if actual, err := field.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got %q from text field; want %q", actual, expected)
			}
		}
	}
}

func chromeVirtualKeyboardNumberInputTest(
	ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, s *testing.State) {
	const (
		activityName = ".MainActivity"
		fieldID      = virtualKeyboardTestAppPkg + ":id/text"
	)

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(ctx)

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

	s.Log("Waiting for vitual keyboard to be ready")
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
	}

	s.Log("Switching to the symbol/number keyboard")
	if err := vkbCtx.TapKeyJS(`"switch to symbols"`)(ctx); err != nil {
		s.Fatal("Failed to tap 'switch to symbols': ", err)
	}
	if err := vkbCtx.WaitLocationStable()(ctx); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to be ready: ", err)
	}

	keys := []string{"1", "2", "#"}
	expected := ""
	for _, key := range keys {
		if err := vkbCtx.TapKey(key)(ctx); err != nil {
			s.Fatalf("Failed to tap %q: %v", key, err)
		}

		expected += key

		// Check the input field after each keystroke to avoid flakiness.
		if err := field.WaitForText(ctx, expected, 30*time.Second); err != nil {
			if actual, err := field.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got %q from text field; want %q", actual, expected)
			}
		}
	}
}

func ChromeVirtualKeyboard(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	a := p.ARC
	cr := p.Chrome
	d := p.UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

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
