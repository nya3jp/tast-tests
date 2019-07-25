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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboard,
		Desc:         "Checks physical keyboard works on Android",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{"ArcKeyboardTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func PhysicalKeyboard(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
	)

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	testTextField := func(activityName, keystrokes, expectedResult string) error {
		act, err := arc.NewActivity(a, pkg, activityName)
		if err != nil {
			return errors.Wrapf(err, "failed to create a new activity %q", activityName)
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return errors.Wrapf(err, "failed to start the activity %q", activityName)
		}

		const (
			fieldID  = "org.chromium.arc.testapp.keyboard:id/text"
			initText = "hello"
		)

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

		// In order to use GetText() after timeout, we should have shorter timeout than ctx.
		if err := d.Object(ui.ID(fieldID), ui.Text(expectedResult)).WaitForExists(ctx, 2*time.Minute); err != nil {
			actual, err := field.GetText(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get text")
			}
			return errors.Errorf("got input %q from field after typing %q", actual, keystrokes)
		}

		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "failed to stop the activity %q", activityName)
		}

		return nil
	}

	if err := testTextField(".MainActivity", "google", "google"); err != nil {
		s.Fatal("Failed to type in normal text field: ", err)
	}

	if err := testTextField(".NullEditTextActivity", "abcdef\b\b\bghi", "abcghi"); err != nil {
		s.Fatal("Failed to type in TYPE_NULL text field: ", err)
	}
}
