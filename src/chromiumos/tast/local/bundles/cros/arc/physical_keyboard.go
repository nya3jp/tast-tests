// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
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
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboard,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks physical keyboard works on Android",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Val:               stablePkTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               stablePkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "unstable",
			Val:               unstablePkTests,
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "unstable_vm",
			Val:               unstablePkTests,
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
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

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
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

func physicalKeyboardBasicEditingOnFrenchTest(ctx context.Context, st pkTestState, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	imePrefix, err := ime.Prefix(ctx, st.tconn)
	if err != nil {
		s.Fatal("Failed to get the IME extension prefix: ", err)
	}
	currentImeID, err := ime.CurrentInputMethod(ctx, st.tconn)
	if err != nil {
		s.Fatal("Failed to get the current IME ID: ", err)
	}

	frImeID := imePrefix + ime.FrenchFrance.ID
	if err := ime.AddAndSetInputMethod(ctx, st.tconn, frImeID); err != nil {
		s.Fatal("Failed to switch to the French IME: ", err)
	}
	if err := ime.WaitForInputMethodMatches(ctx, st.tconn, frImeID, 30*time.Second); err != nil {
		s.Fatal("Failed to switch to the French IME: ", err)
	}
	defer ime.RemoveInputMethod(cleanupCtx, st.tconn, frImeID)
	defer ime.SetCurrentInputMethod(cleanupCtx, st.tconn, currentImeID)

	if err := testTextField(ctx, st, s, ".MainActivity", "qwerty", "azerty"); err != nil {
		s.Error("Failed to type in normal text field: ", err)
	}
}

func PhysicalKeyboard(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
	)

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
