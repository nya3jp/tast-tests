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
		fieldID      = pkg + ":id/typed_character"
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

	imeID, err := ime.CurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current ime: ", err)
	}
	imePrefix, err := ime.Prefix(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get ime prefix: ", err)
	}
	defer func(ctx context.Context) {
		if err := ime.SetCurrentInputMethod(ctx, tconn, imeID); err != nil {
			s.Error("Failed to set the default input method: ", err)
		}
	}(cleanupCtx)

	for _, tc := range []struct {
		name     string
		im       *ime.InputMethod
		mappings []struct{ in, out string }
	}{
		{
			name: "QWERTY keyboard",
			im:   nil,
			mappings: []struct{ in, out string }{
				{"q", "q"},
				{"shift+q", "Q"},
			},
		},
		{
			name: "AZERTY keyboard",
			im:   &ime.FrenchFrance,
			mappings: []struct{ in, out string }{
				{"q", "a"},
				{"shift+q", "A"},
				{"5", "("},
				{"shift+5", "5"},
				{"altgr+5", "["},
				{"-", ")"},
				{"altgr+-", "]"},
				// Display values for dead keys are defined in android.view.KeyCharacterMap
				{"[", "\u02c6"},       //  ACCENT_CIRCUMFLEX
				{"shift+[", "\u00a8"}, //  ACCENT_UMLAUT
			},
		},
		{
			name: "JCUKEN keyboard",
			im:   &ime.Russian,
			mappings: []struct{ in, out string }{
				{"q", "й"},
				{"shift+q", "Й"},
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if tc.im != nil {
				if err := ime.AddAndSetInputMethod(ctx, tconn, imePrefix+tc.im.ID); err != nil {
					s.Fatalf("Failed to switch the IME %q: %v", tc.im.Name, err)
				}

				defer func(ctx context.Context) {
					if err := ime.RemoveInputMethod(ctx, tconn, imePrefix+tc.im.ID); err != nil {
						s.Errorf("Failed to remove the IME %q: %v", tc.im.Name, err)
					}
				}(ctx)
			}

			for _, v := range tc.mappings {
				if err := kb.Accel(ctx, v.in); err != nil {
					s.Fatal("Failed to type: ", err)
				}

				if err := d.Object(ui.ID(fieldID), ui.Text(v.out)).WaitForExists(ctx, 10*time.Second); err != nil {
					s.Errorf("Failed to find field %q after typing %q: %v", v.in, v.out, err)
				}
			}
		})
	}
}
