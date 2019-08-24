// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IMESwitchShortcut,
		Desc:         "Chrome's IME switch shortcut can work on an Android app",
		Contacts:     []string{"yhanada@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcKeyboardTest.apk"},
		Pre:          arc.Booted(),
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

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
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
	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.languageSettingsPrivate.addInputMethod(%q);`, intlIMEID), nil); err != nil {
		s.Fatal("Failed to enable US International keyboard: ", err)
	}

	s.Log("Activating US keyboard")
	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.inputMethodPrivate.setCurrentInputMethod(%q);`, usIMEID), nil); err != nil {
		s.Fatal("Failed to activate US keyboard: ", err)
	}

	getCurrentInputMethod := func() (string, error) {
		var ret string
		if err := tconn.EvalPromise(ctx,
			`new Promise(function(resolve, reject) {
	                  chrome.inputMethodPrivate.getCurrentInputMethod(function(id) {
                            resolve(id);
		          });
		        })`, &ret); err != nil {
			return "", errors.Wrap(err, "failed to get current ime")
		}
		return ret, nil
	}
	if imeID, err := getCurrentInputMethod(); err != nil {
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

	if imeID, err := getCurrentInputMethod(); err != nil {
		s.Fatal("Failed to get current ime: ", err)
	} else if imeID != intlIMEID {
		s.Fatal("Failed to switch international keyboard: ", err)
	}
}
