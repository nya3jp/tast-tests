// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyCharacterMap,
		Desc:         "Checks key character map",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      3 * time.Minute,
	})
}

func KeyCharacterMap(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const (
		apk = "ArcKeyCharacterMapTest.apk"
		pkg = "org.chromium.arc.testapp.kcm"
		cls = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	// defer act.Stop(ctx)

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

	if err := kb.Type(ctx, "q"); err != nil {
		s.Fatal("Failed to type: ", err)
	}

	const fieldID = "org.chromium.arc.testapp.kcm:id/typed_character"
	if err := d.Object(ui.ID(fieldID), ui.Text("q")).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}

	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.languageSettingsPrivate.enableLanguage(%q);`, "fr-FR"), nil); err != nil {
		s.Fatal("Failed to enable French: ", err)
	}

	const inputMethodID = "_comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:fr::fra"
	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.languageSettingsPrivate.addInputMethod(%q);`, inputMethodID), nil); err != nil {
		s.Fatal("Failed to enable French IME: ", err)
	}

	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.inputMethodPrivate.setCurrentInputMethod(%q);`, inputMethodID), nil); err != nil {
		s.Fatal("Failed to activate French IME: ", err)
	}

	if err := kb.Type(ctx, "q"); err != nil {
		s.Fatal("Failed to type: ", err)
	}
	if err := d.Object(ui.ID(fieldID), ui.Text("a")).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}

	// TODO(tetsui): Test Russian keyboard
	// TODO(tetsui): Check keycode as well
}
