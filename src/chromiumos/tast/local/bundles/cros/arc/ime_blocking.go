// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImeBlocking,
		Desc:         "Checks if IME blocking works on ARC",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcImeBlockingTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      3 * time.Minute,
	})
}

func ImeBlocking(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const (
		apk = "ArcImeBlockingTest.apk"
		pkg = "org.chromium.arc.testapp.imeblocking"
		cls = "org.chromium.arc.testapp.imeblocking.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", fmt.Sprintf("%s/%s", pkg, cls)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	const (
		fieldID  = "org.chromium.arc.testapp.imeblocking:id/text"
		buttonID = "org.chromium.arc.testapp.imeblocking:id/button"
	)
	s.Log("Setting up app's initial state")
	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	const (
		keystrokes1        = "hello"
		keystrokes2        = "world"
		keystrokesRejected = "goodbye"
	)

	if err := kb.Type(ctx, keystrokes1); err != nil {
		s.Fatalf("Failed to type %q: %v", keystrokes1, err)
	}

	s.Log("Opening a dialog")
	button := d.Object(ui.ID(buttonID))
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	if err := d.Object(ui.Text("OK"), ui.PackageName(pkg)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for a dialog: ", err)
	}

	if err := kb.Type(ctx, keystrokesRejected); err != nil {
		s.Fatalf("Failed to type %q: %v", keystrokesRejected, err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type Enter: ", err)
	}

	s.Log("Waiting for the dialog to close")
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}

	if err := kb.Type(ctx, keystrokes2); err != nil {
		s.Fatalf("Failed to type %q: %v", "world", err)
	}

	if err := d.Object(ui.ID(fieldID), ui.Text(keystrokes1+keystrokes2)).WaitForExists(ctx, 2*time.Minute); err != nil {
		if actual, err := field.GetText(ctx); err != nil {
			s.Fatal("Failed to get text: ", err)
		} else {
			s.Fatalf("Got input %q from field after typing %q", actual, keystrokes1+keystrokes2)
		}
	}
}
