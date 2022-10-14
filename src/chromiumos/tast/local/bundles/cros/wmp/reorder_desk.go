// Copyright 2021 The ChromiumOS Authors
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
	keyboardInput
)

// The keyboard shortcuts for reordering desks.
const (
	// The shortcut to move a desk to left.
	moveLeft = "Ctrl+Left"
	// The shortcut to move a desk to right.
	moveRight = "Ctrl+Right"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReorderDesk,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that reordering desk by drag & drop and keyboard shortcuts works well",
		Contacts: []string{
			"yongshun@chromium.org",
			"zxdan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Drag desk to reorder.
			Value: "screenplay-f64b4ed7-ca0e-4ea4-85b9-99254079ebde",
		}},
		Fixture: "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "mouse",
				Val:  mouseInput,
			},
			{
				Name: "touch",
				Val:  touchInput,
			},
			{
				Name: "keyboard",
				Val:  keyboardInput,
			},
		},
	})
}

// ReorderDesk tests the reordering of desks by using mouse, touch screen and keyboard.
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

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	// When there is only one desk (no desk mini view), change the first desk's name
	// to prevent the name from being changed by reordering and create another desk
	// with user defined name. Otherwise, confirm the existing desk names are expected.
	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ui)
	if err != nil {
		s.Fatal("Failed to get desk mini views info: ", err)
	}

	if len(deskMiniViewsInfo) == 0 {
		// Change the first desk'name.
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
	}

	ime := s.Param().(inputMethod)

	if ime != keyboardInput {
		// Move the 'First Desk' to the second position.
		if err := reorderDeskByDragAndDrop(ctx, tconn, ime, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk to the second by drag and drop: ", err)
		}
		// Now, the 'First Desk' should be at the second position and the 'Second Desk' should be at the first position.
		// Move the 'First Desk' back to the first position.
		if err := reorderDeskByDragAndDrop(ctx, tconn, ime, "First Desk", "Second Desk"); err != nil {
			s.Fatal("Failed to move the first desk back to first by drag and drop: ", err)
		}
	} else {
		// Move the highlight to the first desk preview.
		if err := kb.AccelAction("Tab")(ctx); err != nil {
			s.Fatal("Failed to move the highlight to the first desk: ", err)
		}

		// Move the 'First Desk' to the second position.
		if err := reorderDeskByKeyboard(ctx, tconn, moveRight); err != nil {
			s.Fatal("Failed to move the first desk to the second position by keyboard: ", err)
		}
		if err := reorderDeskByKeyboard(ctx, tconn, moveLeft); err != nil {
			s.Fatal("Failed to move the first desk back to the first position by keyboard: ", err)
		}
	}
}

func reorderDeskByDragAndDrop(ctx context.Context, tconn *chrome.TestConn, ime inputMethod, sourceName, targetName string) error {
	ui := uiauto.New(tconn)

	sourceDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", sourceName))
	targetDeskMiniView := nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", targetName))

	// Get the locations of source desk and target desk.
	sourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of source desk %s:", sourceName)
	}
	targetDeskMiniViewLoc, err := ui.Location(ctx, targetDeskMiniView)
	if err != nil {
		return errors.Wrapf(err, "failed to get the location of target desk %s:", targetName)
	}

	// Reorder the desk by drag and drop with given input method.
	switch ime {
	case mouseInput:
		// Reorders desks by mouse.
		pc := pointer.NewMouse(tconn)
		defer pc.Close()

		if err := pc.Drag(
			sourceDeskMiniViewLoc.CenterPoint(),
			pc.DragTo(targetDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag and drop desk by mouse")
		}
	case touchInput:
		// Reorders desks by touch.
		tc, err := touch.New(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get touch screen")
		}
		defer tc.Close()

		if err := tc.Swipe(
			sourceDeskMiniViewLoc.CenterPoint(),
			// Long press on the desk mini view to activate reordering.
			tc.Hold(time.Second),
			tc.SwipeTo(targetDeskMiniViewLoc.CenterPoint(), 3*time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to drag and drop desk by touch")
		}
	default:
		return errors.Errorf("invalid input method: %v", ime)
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

// reorderDeskByKeyboard simulates reordering desks by using keyboard shortcuts.
func reorderDeskByKeyboard(ctx context.Context, tconn *chrome.TestConn, shortcut string) error {
	ui := uiauto.New(tconn)

	sourceDeskMiniView := nodewith.ClassName("DeskMiniView").Name("Desk: First Desk")
	targetDeskMiniView := nodewith.ClassName("DeskMiniView").Name("Desk: Second Desk")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a keyboard")
	}
	defer kb.Close()

	// Reorders desks by keyboard.
	targetDeskMiniViewLoc, err := ui.Location(ctx, targetDeskMiniView)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of Second Desk")
	}

	if err := kb.AccelAction(shortcut)(ctx); err != nil {
		return errors.Wrapf(err, "failed to use keyboard shortcut: %s", shortcut)
	}

	// The new desk location should be at the target desk position.
	newSourceDeskMiniViewLoc, err := ui.Location(ctx, sourceDeskMiniView)
	if err != nil {
		return errors.Wrap(err, "failed to get the new location of the First Desk")
	}
	if *newSourceDeskMiniViewLoc != *targetDeskMiniViewLoc {
		return errors.New("First Desk is not reordered to the target position")
	}

	return nil
}
