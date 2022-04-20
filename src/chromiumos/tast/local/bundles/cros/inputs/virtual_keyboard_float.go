// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"math"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Validity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Params: []testing.Param{
			{
				Name:      "tablet",
				Pre:       pre.VKEnabledTabletReset,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:      "clamshell",
				Pre:       pre.VKEnabledClamshellReset,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:      "tablet_fixture",
				Fixture:   fixture.TabletVK,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "clamshell_fixture",
				Fixture:   fixture.ClamshellVK,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, s.TestName())

	vkbCtx := vkb.NewContext(cr, tconn)

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	if err := vkbCtx.SetFloatingMode(uc, true)(ctx); err != nil {
		s.Fatal("Failed to set VK to floating mode: ", err)
	}

	validateDragVKAction := func(ctx context.Context) error {
		// Get current center point of drag button.
		dragLoc, err := uiauto.New(tconn).Location(ctx, vkb.DragPointFinder)
		if err != nil {
			return errors.Wrap(err, "failed to find drag point")
		}
		dragPoint := dragLoc.CenterPoint()

		// Drag float vk to new position.
		destinationPoint := coords.NewPoint(dragPoint.X-100, dragPoint.Y-100)
		if err := mouse.Drag(tconn, dragPoint, destinationPoint, time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag float window")
		}

		// Get new center point of drag button.
		newDragLoc, err := uiauto.New(tconn).Location(ctx, vkb.DragPointFinder)
		if err != nil {
			return errors.Wrap(err, "failed to find drag point")
		}
		newDragPoint := newDragLoc.CenterPoint()

		// When dragging the virtual keyboard to a given location, the actual location it lands on can be slightly different.
		// e.g. When dragging the virtual keyboard to (1016,762), it can end up at (1015, 762).
		if math.Abs(float64(newDragPoint.X-destinationPoint.X)) > 3 || math.Abs(float64(newDragPoint.Y-destinationPoint.Y)) > 3 {
			return errors.Wrapf(err, "failed to drag float VK or it did not land at desired location. got: %v, want: %v", newDragPoint, destinationPoint)
		}

		// Wait for resize handler to be shown.
		resizeHandleFinder := vkb.NodeFinder.Name("resize keyboard, double tap then drag to resize the keyboard").Role(role.Button)

		// Resizing float vk on some boards are flaky.
		// Thus only check the handler is shown.
		return uiauto.New(tconn).WaitUntilExists(resizeHandleFinder.First())(ctx)
	}

	if err := uiauto.UserAction(
		"Drag floating VK",
		validateDragVKAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureFloatVK,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate dragging floating VK: ", err)
	}
}
