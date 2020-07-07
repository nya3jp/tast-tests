// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard", "--force-tablet-mode=touch_view", "--enable-features=VirtualKeyboardFloatingDefault"))
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

	if err := mouse.Move(ctx, tconn, resizeTopLeftHandler, 0); err != nil {
		s.Fatal("Failed to move to the resize start location: ", err)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press left button: ", err)
	}

	s.Log("Wait for float vk to be resizing")
	if err := kconn.WaitForExprFailOnErrWithTimeout(ctx, `document.querySelector("body.is-resizing")`, 3*time.Second); err != nil {
		s.Fatal(err, "failed to wait for resizing started: ", err)
	}

	if err := mouse.Move(ctx, tconn, resizeToPoint, 1*time.Second); err != nil {
		s.Fatal("Failed to move to the resize end location: ", err)
	}

	// Resizing can be a bit delayed, polling to wait.
	// New position after resizing can be not precisely verified. Simply check new drag point moves in the desired direction.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get new top left resize handler button.
		newResizeTopLeftHandler, err := elementCenterPoint(ctx, tconn, params)
		if err != nil {
			return errors.Wrap(err, "failed to get new resize handler")
		}
		if resizeTopLeftHandler.X <= newResizeTopLeftHandler.X || resizeTopLeftHandler.Y <= newResizeTopLeftHandler.Y {
			return errors.Errorf("top left resize handle old position: %s. New position: %s", resizeTopLeftHandler, newResizeTopLeftHandler)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Error("Failed to wait for float vk resized: ", err)
	}
}

func elementCenterPoint(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (coords.Point, error) {
	element, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to find element")
	}
	return element.Location.CenterPoint(), nil
}
