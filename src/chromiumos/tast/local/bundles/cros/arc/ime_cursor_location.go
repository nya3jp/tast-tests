// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IMECursorLocation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Cursor location is correctly sent to Chrome IME",
		Contacts:     []string{"hirokisato@chromium.org", "yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func IMECursorLocation(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	s.Log("Starting app")

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
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

	var imeID string
	if imeID, err = ime.CurrentInputMethod(ctx, tconn); err != nil {
		s.Fatal("Failed to get current ime: ", err)
	}
	var imePrefix string
	if imePrefix, err = ime.Prefix(ctx, tconn); err != nil {
		s.Fatal("Failed to get ime prefix: ", err)
	}

	jpIMEID := imePrefix + ime.Japanese.ID

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		if err := ime.SetCurrentInputMethod(ctx, tconn, imeID); err != nil {
			s.Log("Failed to activate US keyboard: ", err)
		}
		if err := ime.RemoveInputMethod(ctx, tconn, jpIMEID); err != nil {
			s.Log("Failed to disable US International keyboard: ", err)
		}
	}(cleanupCtx)

	s.Log("Switching to the JP IME")
	if err := ime.AddAndSetInputMethod(ctx, tconn, jpIMEID); err != nil {
		s.Fatal("Failed to switch to the Japanese IME: ", err)
	}
	if err := ime.WaitForInputMethodMatches(ctx, tconn, jpIMEID, 30*time.Second); err != nil {
		s.Fatal("Failed to switch to the Japanese IME: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// type "a" and "i", and press a space wtice to make sure that candidate window is opened.
	if err := kb.TypeSequence(ctx, []string{"a", "i", "space", "space"}); err != nil {
		s.Fatal("Failed to type: ", err)
	}

	uia := uiauto.New(tconn)
	candidateWindowFinder := nodewith.HasClass("CandidateWindowView").Role(role.Window)
	candidateWindowBoundsDp, err := uia.Location(ctx, candidateWindowFinder)
	if err != nil {
		s.Fatal("Failed to get location: ", err)
	}

	editTextBoundsPx, err := field.GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get location: ", err)
	}

	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}

	dsf, err := dispInfo.GetEffectiveDeviceScaleFactor()
	if err != nil {
		s.Fatal("Failed to get the effective device scale factor: ", err)
	}

	editTextBoundsDp := coords.ConvertBoundsFromPXToDP(editTextBoundsPx, dsf)
	if !validateBoundsRelationship(editTextBoundsDp, *candidateWindowBoundsDp) {
		s.Fatalf("Failed to validate bounds relationship. candidate window: %+v, edit text: %+v", candidateWindowBoundsDp, editTextBoundsDp)
	}
}

// validateBoundsRelationship checks that the candidate window bounds is almost adjacent to the edit text bounds,
// but the candidate window doesn't hide the edit text.
func validateBoundsRelationship(editTextBounds, candidateWindowBounds coords.Rect) bool {
	margin := editTextBounds.Height / 5
	expanded := editTextBounds.WithInset(0, -margin)
	shrinked := editTextBounds.WithInset(0, margin)

	return !candidateWindowBounds.Intersection(expanded).Empty() &&
		candidateWindowBounds.Intersection(shrinked).Empty()
}
