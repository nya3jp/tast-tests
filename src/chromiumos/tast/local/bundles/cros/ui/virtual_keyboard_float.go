// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		Desc:         "Sanity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.VKEnabled(),
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	err = vkb.SwitchToFloatMode(ctx, tconn)
	if err != nil {
		s.Fatal("Switch to floating layout failed: ", err)
	}

	tabletEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current ui mode: ", err)
	}

	var pc pointer.Controller
	if tabletEnabled {
		var err error
		if pc, err = pointer.NewTouchController(ctx, tconn); err != nil {
			s.Fatal("Failed to create a touch controller")
		}
	} else {
		pc = pointer.NewMouseController(tconn)
	}
	defer pc.Close()

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
	if err := pointer.Drag(ctx, pc, dragPoint, destinationPoint, time.Second); err != nil {
		s.Fatal("Failed to drag float window: ", err)
	}

	// Get current center point of drag button.
	newDragPoint, err := elementCenterPoint(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}

	if !newDragPoint.Equals(destinationPoint) {
		s.Errorf("Failed to drag float VK or it did not land at desired location. got: %v, want: %v", newDragPoint, destinationPoint)
	}

	// Wait for resize handler.
	params = ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "resize keyboard, double tap then drag to resize the keyboard",
	}

	// Get top left resize handler button.
	resizeTopLeftHandler, err := elementCenterPoint(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get resize handler: ", err)
	}

	// Drag top left to resize layout.
	resizeToPoint := coords.NewPoint(resizeTopLeftHandler.X-100, resizeTopLeftHandler.Y-100)
	if err := pointer.Drag(ctx, pc, resizeTopLeftHandler, resizeToPoint, time.Second); err != nil {
		s.Fatal("Failed to resize vk: ", err)
	}

	// Get new top left resize handler button.
	newResizeTopLeftHandler, err := elementCenterPoint(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get new resize handler: ", err)
	}

	// New position after resizing can be not precisely verified. Simply check new drag point moves in the desired direction.
	if resizeTopLeftHandler.X <= newResizeTopLeftHandler.X || resizeTopLeftHandler.Y <= newResizeTopLeftHandler.Y {
		s.Errorf("Failed to resize float VK. Top left resize handle old position: %s. New position: %s", resizeTopLeftHandler, newResizeTopLeftHandler)
	}
}

func elementCenterPoint(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (coords.Point, error) {
	element, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to find element")
	}
	return element.Location.CenterPoint(), nil
}
