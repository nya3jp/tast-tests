// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
)

// CreateNewDesk requests Ash to create a new Virtual Desk which would fail if
// the maximum number of desks have been reached.
func CreateNewDesk(ctx context.Context, tconn *chrome.TestConn) error {
	success := false
	if err := tconn.Call(ctx, &success, "tast.promisify(chrome.autotestPrivate.createNewDesk)"); err != nil {
		return err
	}
	if !success {
		return errors.New("failed to create a new desk")
	}
	return nil
}

// CleanUpDesks removes all but one desk.
func CleanUpDesks(ctx context.Context, tconn *chrome.TestConn) error {
	// Ensure not in overview, to work around https://crbug.com/1309220.
	// TODO(https://crbug.com/1309220): Remove this when the bug is fixed.
	if err := SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to ensure not in overview")
	}

	// To remove all but one desk, we invoke chrome.autotestPrivate.removeActiveDesk
	// repeatedly until it returns false. It is guaranteed to return true as long as
	// there is more than one desk (see DesksController::CanRemoveDesks in chromium).
	for success := true; success; {
		if err := tconn.Call(ctx, &success, "tast.promisify(chrome.autotestPrivate.removeActiveDesk)"); err != nil {
			return errors.Wrap(err, "failed to remove desk")
		}
	}
	return nil
}

// ActivateDeskAtIndex requests Ash to activate the Virtual Desk at the given index.
// It waits for the desk-switch animation to complete. This call will fail if index is
// invalid, or its the index of the already active desk.
func ActivateDeskAtIndex(ctx context.Context, tconn *chrome.TestConn, index int) error {
	success := false
	if err := tconn.Call(ctx, &success, "tast.promisify(chrome.autotestPrivate.activateDeskAtIndex)", index); err != nil {
		return err
	}
	if !success {
		return errors.Errorf("failed to activate desk at index %v", index)
	}
	return nil
}

// RemoveActiveDesk requests Ash to remove the currently active desk and waits for the
// desk-removal animation to complete. This call will fail if the currently active desk
// is the last available desk which cannot be removed.
func RemoveActiveDesk(ctx context.Context, tconn *chrome.TestConn) error {
	success := false
	if err := tconn.Call(ctx, &success, "tast.promisify(chrome.autotestPrivate.removeActiveDesk)"); err != nil {
		return err
	}
	if !success {
		return errors.New("failed to remove the active desk")
	}
	return nil
}

// ActivateAdjacentDesksToTargetIndex requests Ash to keep activating the adjacent
// Virtual Desk until the one at the given index is reached. It waits for the chain
// of desk-switch animations to complete. This call will fail if index is invalid,
// or it is the index of the already active desk.
func ActivateAdjacentDesksToTargetIndex(ctx context.Context, tconn *chrome.TestConn, index int) error {
	success := false
	if err := tconn.Call(ctx, &success, "tast.promisify(chrome.autotestPrivate.activateAdjacentDesksToTargetIndex)",
		index); err != nil {
		return err
	}
	if !success {
		return errors.Errorf("failed to activate desk at index %v", index)
	}
	return nil
}

// FindDeskMiniViews returns a list of DeskMiniView nodes.
// TODO(crbug/1251558): use autotest api to get the number of desks instead.
func FindDeskMiniViews(ctx context.Context, ac *uiauto.Context) ([]uiauto.NodeInfo, error) {
	deskMiniViews := nodewith.ClassName("DeskMiniView")
	deskMiniViewsInfo, err := ac.NodesInfo(ctx, deskMiniViews)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find all desk mini views")
	}
	return deskMiniViewsInfo, nil
}

// FindDeskTemplates returns a list of saved desk nodes.
func FindDeskTemplates(ctx context.Context, ac *uiauto.Context) ([]uiauto.NodeInfo, error) {
	savedDeskItemView := nodewith.ClassName("SavedDeskItemView")
	savedDeskItemViewInfo, err := ac.NodesInfo(ctx, savedDeskItemView)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find SavedDeskItemView")
	}
	return savedDeskItemViewInfo, nil
}

// DeleteAllDeskTemplates cleans up desk templates by using a series of mouse and keyboard events to delete each template.
func DeleteAllDeskTemplates(ctx context.Context, ac *uiauto.Context, tconn *chrome.TestConn) error {
	libraryButton := nodewith.Name("Library")
	savedDeskGridView := nodewith.ClassName("SavedDeskLibraryView")
	closeButton := nodewith.ClassName("CloseButton").Name("Delete")
	deleteDialog := nodewith.ClassName("SavedDeskDialog")
	// See if there are desk templates to delete.
	for found, _ := ac.IsNodeFound(ctx, libraryButton); found; found, _ = ac.IsNodeFound(ctx, libraryButton) {
		if err := uiauto.Combine(
			"Go into Library",
			ac.LeftClick(libraryButton),
			ac.WaitUntilExists(savedDeskGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to get into library view for clean up")
		}
		deskTemplatesInfo, _ := FindDeskTemplates(ctx, ac)
		for i := 0; i < len(deskTemplatesInfo); i++ {
			firstDeskTemplate := nodewith.ClassName("SavedDeskItemView").First()
			if err := uiauto.Combine(
				"Move mouse to location",
				ac.MouseMoveTo(firstDeskTemplate, 0),
			)(ctx); err != nil {
				return errors.Wrap(err, "fail to mouse over to desk template")
			}
			mouse.Press(tconn, mouse.LeftButton)
			kb, err := input.Keyboard(ctx)
			if err != nil {
				return errors.Wrap(err, "failed create keyboard")
			}
			defer kb.Close()
			if err := uiauto.Combine(
				"Delete desk templates",
				ac.WaitUntilExists(closeButton),
				ac.LeftClick(closeButton),
				ac.WaitUntilExists(deleteDialog),
				kb.AccelAction("Enter"),
			)(ctx); err != nil {
				return errors.Wrap(err, "fail to delete desk template")
			}
		}
		// Exits overview mode.
		if err := SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "fail to get out of overview mode")
		}
		// Re-enter overview mode and continue deleting saved desk templates.
		if err := SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "fail to set overview mode")
		}
	}
	return nil
}
