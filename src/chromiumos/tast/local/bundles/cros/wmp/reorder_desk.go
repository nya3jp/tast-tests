// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type inputMethod int

const (
	mouseInput inputMethod = iota
	touchInput
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ReorderDesk,
		Desc: "Checks that reordering desk by drag & drop and keyboard shortcuts works well",
		Contacts: []string{
			"zxdan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "mouse",
				Val:  mouseInput,
			},
			{
				Name: "touch",
				Val:  touchInput,
			},
		},
	})
}

// ReorderDesk tests the reordering of desks by using mouse and touch screen.
func ReorderDesk(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	// Change the first desk's name to prevent the name from being changed by reordering.
	zeroStateDefaultDeskButton := nodewith.ClassName("ZeroStateDefaultDeskButton")
	firstDeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 1")
	if err := uiauto.Combine(
		"change the first desk's name",
		ui.LeftClick(zeroStateDefaultDeskButton),
		// The focus on the new desk should be on the desk name field.
		ui.WaitUntilExists(firstDeskNameView.Focused()),
		kb.TypeAction("First Desk"),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to change the name of the first desk: ", err)
	}

	// Creates a new desks with user defined name.
	addDeskButton := nodewith.ClassName("ExpandedDesksBarButton")
	newDeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	if err := uiauto.Combine(
		"create a new desk",
		ui.LeftClick(addDeskButton),
		// The focus on the new desk should be on the desk name field.
		ui.WaitUntilExists(newDeskNameView.Focused()),
		kb.TypeAction("Second Desk"),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to create the second desk: ", err)
	}

	// Here, we need to do some operations to get the name of desk nodes updated.
	// Otherwise, the name of second desk is still Desk 2.
	// Exit Overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit the Overview: ", err)
	}
	// Re-enter Overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter the Overview: ", err)
	}

	ime := s.Param().(inputMethod)

	// Move the 'First Desk' to the second position.
	switch ime {
	case mouseInput:
		if err := reorderDeskByMouse(ctx, tconn, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk to the second position by mouse: ", err)
		}
	case touchInput:
		if err := reorderDeskByTouch(ctx, tconn, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk to the second position by touch: ", err)
		}
	default:
		err := errors.Errorf("invalid input method: %v", ime)
		s.Fatal("Failed to move the first desk to the second position: ", err)
	}

	// Now, the 'First Desk' should be at the second position and the 'Second Desk' should be at the first position.
	// Move the 'First Desk' back to the first position.
	switch ime {
	case mouseInput:
		if err := reorderDeskByMouse(ctx, tconn, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk to the second position by mouse: ", err)
		}
	case touchInput:
		if err := reorderDeskByTouch(ctx, tconn, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk to the second position by touch: ", err)
		}
	default:
		err := errors.Errorf("invalid input method: %v", ime)
		s.Fatal("Failed to move the first desk to the second position: ", err)
	}
}

// reorderDeskByMouse simulates reordering desks by drag and drop with mouse cursor.
func reorderDeskByMouse(ctx context.Context, tconn *chrome.TestConn, sourceName, targetName string) error {
	ui := uiauto.New(tconn)

	sourceDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", sourceName))
	targetDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", targetName))

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Reorders desks by mouse.
	sourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of source desk %s:", sourceName)
	}
	targetDeskMiniViewLoc, err := ui.Location(ctx, targetDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of target desk %s:", targetName)
	}
	if err := pc.Drag(
		sourceDeskMiniViewLoc.CenterPoint(),
		pc.DragTo(targetDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag and drop desk by mouse")
	}

	// The new desk location should be at the target desk position.
	newSourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the new location of the source desk %s", sourceName)
	}
	if *newSourceDeskMiniViewLoc != *targetDeskMiniViewLoc {
		return errors.New("source desk is not reordered to the target position")
	}

	return nil
}

// reorderDeskByTouch simulates reordering desks by drag and drop with finger.
func reorderDeskByTouch(ctx context.Context, tconn *chrome.TestConn, sourceName, targetName string) error {
	ui := uiauto.New(tconn)

	sourceDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", sourceName))
	targetDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", targetName))

	tc, err := touch.New(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get touch screen")
	}
	defer tc.Close()

	// Reorders desks by touch.
	sourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of source desk %s:", sourceName)
	}
	targetDeskMiniViewLoc, err := ui.Location(ctx, targetDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of target desk %s:", targetName)
	}
	if err := tc.Swipe(
		sourceDeskMiniViewLoc.CenterPoint(),
		// Long press on the desk mini view to activate reordering.
		tc.Hold(time.Second),
		tc.SwipeTo(targetDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag and drop desk by touch")
	}

	// The new desk location should be at the target desk position.
	newSourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the new location of the source desk %s", sourceName)
	}
	if *newSourceDeskMiniViewLoc != *targetDeskMiniViewLoc {
		return errors.New("source desk is not reordered to the target position")
	}

	return nil
}
