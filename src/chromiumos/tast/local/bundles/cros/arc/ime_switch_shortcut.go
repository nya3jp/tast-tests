// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IMESwitchShortcut,
		Desc:         "Chrome's IME switch shortcut can work on an Android app",
		Contacts:     []string{"yhanada@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func IMESwitchShortcut(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"

		usIMEID   = "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:us::eng"
		intlIMEID = "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:us:intl:eng"
	)

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	s.Log("Setting up app's initial state")
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

	s.Log("Enabling US International keyboard")
	if err := ime.AddInputMethod(ctx, tconn, intlIMEID); err != nil {
		s.Fatal("Failed to enable US International keyboard: ", err)
	}

	s.Log("Activating US keyboard")
	if err := ime.SetCurrentInputMethod(ctx, tconn, usIMEID); err != nil {
		s.Fatal("Failed to activate US keyboard: ", err)
	}

	if imeID, err := ime.GetCurrentInputMethod(ctx, tconn); err != nil {
		s.Fatal("Failed to get current ime: ", err)
	} else if imeID != usIMEID {
		s.Fatal("Failed to activate US keyboard: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Press Ctrl-Space
	if err := kb.Accel(ctx, "Ctrl+Space"); err != nil {
		s.Fatal("Failed to send Ctrl-Space: ", err)
	}

	if imeID, err := ime.GetCurrentInputMethod(ctx, tconn); err != nil {
		s.Fatal("Failed to get current ime: ", err)
	} else if imeID != intlIMEID {
		s.Fatal("Failed to switch international keyboard: ", err)
	}
}
