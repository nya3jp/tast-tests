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
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func PhysicalKeyboard(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	testTextField := func(activityName, keystrokes, expectedResult string) error {
		act, err := arc.NewActivity(a, pkg, activityName)
		if err != nil {
			return errors.Wrapf(err, "failed to create a new activity %q", activityName)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start the activity %q", activityName)
		}
		defer act.Stop(ctx, tconn)

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

		if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expectedResult, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for text")
		}

		return nil
	}

	if err := testTextField(".MainActivity", "google", "google"); err != nil {
		s.Error("Failed to type in normal text field: ", err)
	}

	if err := testTextField(".NullEditTextActivity", "abcdef\b\b\bghi", "abcghi"); err != nil {
		s.Error("Failed to type in TYPE_NULL text field: ", err)
	}
}
