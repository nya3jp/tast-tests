// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		Desc:         "Validity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("VirtualKeyboardFloatingDefault"), chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	vkbCtx := vkb.NewContext(ctx, cr, tconn)

	if err := vkbCtx.ShowVirtualKeyboardAction()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	kconn, err := vkbCtx.UIConn()
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	dragPointFinder := vkb.NodeFinder(
		nodewith.Role(role.Button).Name("move keyboard, double tap then drag to reposition the keyboard"))
	dragPoint, err := elementCenterPoint(ctx, tconn, dragPointFinder)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}
	// Get current center point of drag button.

	// Drag float vk to new position.
	destinationPoint := coords.NewPoint(dragPoint.X-100, dragPoint.Y-100)
	if err := mouse.Drag(ctx, tconn, dragPoint, destinationPoint, time.Second); err != nil {
		s.Fatal("Failed to drag float window: ", err)
	}

	// Get current center point of drag button.
	newDragPoint, err := elementCenterPoint(ctx, tconn, dragPointFinder)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}

	// When dragging the virtual keyboard to a given location, the actual location it lands on can be slightly different.
	// e.g. When dragging the virtual keyboard to (1016,762), it can end up at (1015, 762).
	if math.Abs(float64(newDragPoint.X-destinationPoint.X)) > 3 || math.Abs(float64(newDragPoint.Y-destinationPoint.Y)) > 3 {
		s.Fatalf("Failed to drag float VK or it did not land at desired location. got: %v, want: %v", newDragPoint, destinationPoint)
	}

	// Wait for resize handler to be shown.
	resizeHandleFinder := vkb.NodeFinder(nodewith.Name("resize keyboard, double tap then drag to resize the keyboard").Role(role.Button))

	// Resizing float vk on some boards are flaky.
	// Thus only check the handler is shown.
	uiauto.New(tconn).WaitUntilExists(resizeHandleFinder)
}

func elementCenterPoint(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder) (coords.Point, error) {
	elementLoc, err := uiauto.New(tconn).Location(ctx, finder)
	if err != nil {
		return coords.Point{}, errors.Wrapf(err, "failed to get location of %v", finder)
	}
	return elementLoc.CenterPoint(), nil
}
