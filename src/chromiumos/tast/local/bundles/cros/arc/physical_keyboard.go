// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// pkTestState is a collection of objects needs to run the physical keyboard tests.
type pkTestState struct {
	tconn *chrome.TestConn
	a     *arc.ARC
	d     *ui.Device
	kb    *input.KeyboardEventWriter
}

// pkTestParams represents the name of the test and the function to call.
type pkTestParams struct {
	name string
	fn   func(context.Context, pkTestState, *testing.State)
}

var stablePkTests = []pkTestParams{
	{"Basic editing", physicalKeyboardBasicEditingTest},
	{"Editing on TYPE_NULL", physicalKeyboardOnTypeNullTextFieldTest},
}

var unstablePkTests = []pkTestParams{
	{"Basic editing with non-qwerty", physicalKeyboardBasicEditingOnFrenchTest},
	{"All keycodes", physicalKeyboardAllKeycodesTypingTest},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboard,
		Desc:         "Checks physical keyboard works on Android",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Val:               stablePkTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               stablePkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "unstable",
			Val:               unstablePkTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "unstable_vm",
			Val:               unstablePkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func testTextField(ctx context.Context, st pkTestState, s *testing.State, activity, keystrokes, expectedResult string) error {
	const (
		pkg      = "org.chromium.arc.testapp.keyboard"
		fieldID  = pkg + ":id/text"
		initText = "hello"
	)

	a := st.a
	tconn := st.tconn
	d := st.d
	kb := st.kb

	act, err := arc.NewActivity(a, pkg, activity)
	if err != nil {
		return errors.Wrapf(err, "failed to create a new activity %q", activity)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start the activity %q", activity)
	}
	defer act.Stop(ctx, tconn)

	if err := d.Object(ui.ID(fieldID), ui.Text(initText)).WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to find field")
	}

	field := d.Object(ui.ID(fieldID))
	if err := field.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click field")
	}
	if err := field.SetText(ctx, ""); err != nil {
		return errors.Wrap(err, "failed to empty field")
	}

	if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to focus on field")
	}

	if err := kb.Type(ctx, keystrokes); err != nil {
		return errors.Wrapf(err, "failed to type %q", keystrokes)
	}

	if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expectedResult, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for text")
	}

	return nil
}

func physicalKeyboardBasicEditingTest(ctx context.Context, st pkTestState, s *testing.State) {
	if err := testTextField(ctx, st, s, ".MainActivity", "google", "google"); err != nil {
		s.Error("Failed to type in normal text field: ", err)
	}
}

func physicalKeyboardOnTypeNullTextFieldTest(ctx context.Context, st pkTestState, s *testing.State) {
	if err := testTextField(ctx, st, s, ".NullEditTextActivity", "abcdef\b\b\bghi", "abcghi"); err != nil {
		s.Error("Failed to type in TYPE_NULL text field: ", err)
	}
}

func physicalKeyboardAllKeycodesTypingTest(ctx context.Context, st pkTestState, s *testing.State) {
	const (
		activityName = ".MainActivity"
		pkg          = "org.chromium.arc.testapp.keyboard"
		fieldID      = "org.chromium.arc.testapp.keyboard:id/text"
	)

	a := st.a
	tconn := st.tconn
	d := st.d
	kb := st.kb

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", activityName, err)
	}

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity before typing:")
	}
	defer act.Stop(ctx, tconn)

	focusField := func() error {
		field := d.Object(ui.ID(fieldID))
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
		if err != nil {
			return errors.Wrap(err, "failed to get the window info")
		}
		if !info.IsVisible || !info.HasFocus || !info.IsActive {
			return errors.New("the app window is not focused")
		}
		if err := field.WaitForExists(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the field")
		}
		if err := d.Object(ui.ID(fieldID), ui.Focused(true)).Exists(ctx); err != nil {
			if err := field.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the field")
			}
		}
		if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to focus the field")
		}
		return nil
	}

	// The channel to make the logcat monitor stop monitoring.
	done := make(chan bool, 1)
	// The channel to make the logcat monitor report any failure in logcat while monitoring.
	result := make(chan error)
	// This goroutine monitors logcat output to find any mojo connection errors of ArcInputMethodService.
	go func(done chan bool) {
		exp := regexp.MustCompile(`ArcInputMethod: Mojo connection error`)

		notFound := make(chan bool, 1)
		isFinished := func() bool {
			select {
			case <-done:
				notFound <- true
				return true
			default:
				return false
			}
		}

		if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp), isFinished); err != nil {
			result <- errors.Wrap(err, "failed to wait for logcat output")
			return
		}
		select {
		case <-notFound:
			result <- nil
		default:
			result <- errors.New("mojo connection error is detected")
		}
	}(done)

	// TODO(b:174259561): There are some edge cases which this test cannot catch the actual failure. For example,
	// case #1:
	//
	// 1. A key (e.g. back button) is pressed that closes the activity
	// 2. focusField() runs before the key #1 is processed by Android and succeeds, because there's no wait
	// 3. A key that leads to ARC crash or mojo disconnection is sent, but it's not sent to Android
	// 4. Because it's not received by Android, it fails to catch the regression
	//
	// case #2:
	//
	// 1. A key that leads to ARC crash or mojo disconnection is sent as a last case
	// 2. As there's no wait after the loop, done is sent to the logcat goroutine before the crash message is received
	// 3. Because the test returns without error, it fails to catch the regression
	s.Log("Start typing all keys")
	defer func() {
		done <- true
	}()
	skipKeys := map[input.EventCode]struct{}{
		// Skip KEY_CAPSLOCK to avoid affecting the following tests by Capslock.
		0x3a: struct{}{},
		// Skip KEY_SYSRQ to avoid launching the screenshot tool. The
		// screenshot tool can cause subsequent tests to fail by intercepting
		// mouse clicks.
		0x63: struct{}{},
		// Skip KEY_LEFTMETA (0x7d) and KEY_RIGHTMETA (0x7e) which are the search keys to avoid confusing the test.
		0x7d: struct{}{},
		0x7e: struct{}{},
	}
	for scancode := input.EventCode(0x01); scancode < 0x220; scancode++ {
		if _, exist := skipKeys[scancode]; exist || (scancode >= 0x80 && scancode < 0x160) {
			continue
		}
		// Check whether the mojo connection is already broken or not.
		select {
		case err := <-result:
			s.Fatalf("ArcInputMethod mojo connection is broken before typing %d: %v", scancode, err)
		default:
		}

		if err := focusField(); err != nil {
			// Cannot find the text field. Restart the activity.
			act.Stop(ctx, tconn)
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatalf("Failed to restart the activity before typing %d: %v", scancode, err)
			}
			if err := focusField(); err != nil {
				s.Fatalf("Failed to focus the field before typing %d: %v", scancode, err)
			}
		}

		s.Logf("Going to type key %d", scancode)
		if err := kb.TypeKey(ctx, scancode); err != nil {
			s.Fatalf("Failed to send the scancode %d: %v", scancode, err)
		}
	}
	s.Log("Finish typing all keys")

	done <- true
	if err := <-result; err != nil {
		s.Fatal("ArcInputMethod mojo connection is broken while typing test: ", err)
	}
}

func physicalKeyboardBasicEditingOnFrenchTest(ctx context.Context, st pkTestState, s *testing.State) {
	imePrefix, err := ime.Prefix(ctx, st.tconn)
	if err != nil {
		s.Fatal("Failed to get the IME extension prefix: ", err)
	}
	currentImeID, err := ime.CurrentInputMethod(ctx, st.tconn)
	if err != nil {
		s.Fatal("Failed to get the current IME ID: ", err)
	}

	frImeID := imePrefix + string(ime.INPUTMETHOD_XKB_FR_FRA)
	if err := ime.AddAndSetInputMethod(ctx, st.tconn, frImeID); err != nil {
		s.Fatal("Failed to switch to the French IME: ", err)
	}
	if err := ime.WaitForInputMethodMatches(ctx, st.tconn, frImeID, 30*time.Second); err != nil {
		s.Fatal("Failed to switch to the French IME: ", err)
	}
	defer ime.RemoveInputMethod(ctx, st.tconn, frImeID)
	defer ime.SetCurrentInputMethod(ctx, st.tconn, currentImeID)

	if err := testTextField(ctx, st, s, ".MainActivity", "qwerty", "azerty"); err != nil {
		s.Error("Failed to type in normal text field: ", err)
	}
}

func PhysicalKeyboard(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	testState := pkTestState{tconn, a, d, kb}
	for _, test := range s.Param().([]pkTestParams) {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			test.fn(ctx, testState, s)
		})
	}
}
