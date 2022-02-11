// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyCharacterMap,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks KeyCharacterMap working in non-US layouts",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func KeyCharacterMap(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC
	d := p.UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const (
		apk          = "ArcKeyCharacterMapTest.apk"
		pkg          = "org.chromium.arc.testapp.kcm"
		activityName = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	checkMapping := func(ctx context.Context, input, output string) {
		fieldID := pkg + ":id/typed_character"

		if err := kb.Type(ctx, input); err != nil {
			s.Fatal("Failed to type: ", err)
		}

		if err := d.Object(ui.ID(fieldID), ui.Text(output)).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Fatal("Failed to find field: ", err)
		}
	}

	var imePrefix string
	if imePrefix, err = ime.Prefix(ctx, tconn); err != nil {
		s.Fatal("Failed to get ime prefix: ", err)
	}

	switchInputMethod := func(ctx context.Context, language, layout string) {
		if err := ime.EnableLanguage(ctx, tconn, language); err != nil {
			s.Fatalf("Failed to enable the language %q: %v", language, err)
		}
		if err := ime.AddInputMethod(ctx, tconn, imePrefix+layout); err != nil {
			s.Fatalf("Failed to enable the IME %q: %v", layout, err)
		}
		if err := ime.SetCurrentInputMethod(ctx, tconn, imePrefix+layout); err != nil {
			s.Fatalf("Failed to activate the IME %q: %v", layout, err)
		}
	}

	removeInputMethod := func(ctx context.Context, language, layout string) {
		if err := ime.RemoveInputMethod(ctx, tconn, imePrefix+layout); err != nil {
			s.Errorf("Failed to enable the IME %q: %v", layout, err)
		}
		if err := ime.DisableLanguage(ctx, tconn, language); err != nil {
			s.Errorf("Failed to enable the language %q: %v", language, err)
		}
	}

	// Check mapping in QWERTY keyboard
	checkMapping(ctx, "q", "q")
	checkMapping(ctx, "Q", "Q")

	// Check mapping in AZERTY keyboard
	defer removeInputMethod(cleanupCtx, "fr-FR", "xkb:fr::fra")
	switchInputMethod(ctx, "fr-FR", "xkb:fr::fra")
	checkMapping(ctx, "q", "a")
	checkMapping(ctx, "Q", "A")

	// Check mapping in the JCUKEN keyboard
	defer removeInputMethod(cleanupCtx, "ru", "xkb:ru::rus")
	switchInputMethod(ctx, "ru", "xkb:ru::rus")
	checkMapping(ctx, "q", "й")
	checkMapping(ctx, "Q", "Й")
}
