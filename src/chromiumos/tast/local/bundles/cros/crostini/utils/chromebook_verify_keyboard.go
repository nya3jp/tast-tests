// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// refer to physical_keyboar.go

// pkTestState is a collection of objects needs to run the physical keyboard tests.
type pkTestState struct {
	tconn *chrome.TestConn
	a     *arc.ARC
	d     *ui.Device
	kb    *input.KeyboardEventWriter
}

// VerifyKeyboard convert physical_keyboard.go's test as function to check keyboard works functionally
func VerifyKeyboard(ctx context.Context, s *testing.State) error {

	s.Log("Verifying keyboard")

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

	if err := physicalKeyboardBasicEditingTest(ctx, testState, s); err != nil {
		return errors.Wrap(err, "failed to verify keyboard")
	}

	return nil
}

func physicalKeyboardBasicEditingTest(ctx context.Context, st pkTestState, s *testing.State) error {
	if err := testTextField(ctx, st, s, ".MainActivity", "google", "google"); err != nil {
		return errors.Wrap(err, "failed to type in normal text field")
	}

	return nil
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
