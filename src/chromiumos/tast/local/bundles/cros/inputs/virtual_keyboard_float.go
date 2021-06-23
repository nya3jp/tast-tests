// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"math"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		Desc:         "Validity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
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
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardFloat.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	vkbCtx := vkb.NewContext(cr, tconn)

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	kconn, err := vkbCtx.UIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	dragPointFinder := vkb.NodeFinder.Role(role.Button).Name("move keyboard, double tap then drag to reposition the keyboard")

	// Get current center point of drag button.
	dragLoc, err := uiauto.New(tconn).Location(ctx, dragPointFinder)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}
	dragPoint := dragLoc.CenterPoint()

	// Drag float vk to new position.
	destinationPoint := coords.NewPoint(dragPoint.X-100, dragPoint.Y-100)
	if err := mouse.Drag(tconn, dragPoint, destinationPoint, time.Second)(ctx); err != nil {
		s.Fatal("Failed to drag float window: ", err)
	}

	// Get new center point of drag button.
	newDragLoc, err := uiauto.New(tconn).Location(ctx, dragPointFinder)
	if err != nil {
		s.Fatal("Failed to find drag point: ", err)
	}
	newDragPoint := newDragLoc.CenterPoint()

	// When dragging the virtual keyboard to a given location, the actual location it lands on can be slightly different.
	// e.g. When dragging the virtual keyboard to (1016,762), it can end up at (1015, 762).
	if math.Abs(float64(newDragPoint.X-destinationPoint.X)) > 3 || math.Abs(float64(newDragPoint.Y-destinationPoint.Y)) > 3 {
		s.Fatalf("Failed to drag float VK or it did not land at desired location. got: %v, want: %v", newDragPoint, destinationPoint)
	}

	// Wait for resize handler to be shown.
	resizeHandleFinder := vkb.NodeFinder.Name("resize keyboard, double tap then drag to resize the keyboard").Role(role.Button)

	// Resizing float vk on some boards are flaky.
	// Thus only check the handler is shown.
	if err := uiauto.New(tconn).WaitUntilExists(resizeHandleFinder.First())(ctx); err != nil {
		s.Fatal("Failed to wait for resize handler to be shown: ", err)
	}
}
