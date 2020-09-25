// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		Desc:         "Sanity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:essential-inputs"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("VirtualKeyboardFloatingDefault"), chrome.ExtraArgs("--enable-virtual-keyboard", "--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	params := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "move keyboard, double tap then drag to reposition the keyboard",
	}

	// Get current center point of drag button.
	dragPoint, err := elementCenterPoint(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}

	// Drag float vk to new position.
	destinationPoint := coords.NewPoint(dragPoint.X-100, dragPoint.Y-100)
	if err := mouse.Drag(ctx, tconn, dragPoint, destinationPoint, time.Second); err != nil {
		s.Fatal("Failed to drag float window: ", err)
	}

	// Get current center point of drag button.
	newDragPoint, err := elementCenterPoint(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}

	// When dragging the virtual keyboard to a given location, the actual location it lands on can be slightly different.
	// e.g. When dragging the virtual keyboard to (1016,762), it can end up at (1015, 762).
	if math.Abs(float64(newDragPoint.X-destinationPoint.X)) > 3 || math.Abs(float64(newDragPoint.Y-destinationPoint.Y)) > 3 {
		s.Fatalf("Failed to drag float VK or it did not land at desired location. got: %v, want: %v", newDragPoint, destinationPoint)
	}

	// Wait for resize handler to be shown.
	params = ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "resize keyboard, double tap then drag to resize the keyboard",
	}

	// Resizing float vk on some boards are flaky.
	// Thus only check the handler is shown.
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for resize handler to be shown: ", err)
	}
}

func elementCenterPoint(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (coords.Point, error) {
	element, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to find element")
	}
	return element.Location.CenterPoint(), nil
}
