// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
)

// SavedDeskType enum distinguishes sved desk type
type SavedDeskType int

const (
	// Template represents desk saved by "Save desk as a template" button
	Template SavedDeskType = iota
	// SaveAndRecall represents desk saved by "Save desk for later" button
	SaveAndRecall
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

// SaveCurrentDesk saves the current desk as `kTemplate` or `kSaveAndRecall`.
// This assumes overview grid is live now.
// TODO(yongshun): Check if in overview mode and error out early if not.
func SaveCurrentDesk(ctx context.Context, ac *uiauto.Context, savedDeskType SavedDeskType, savedDeskName string) error {
	var saveDeskButton *nodewith.Finder
	var savedDeskGridView *nodewith.Finder
	if savedDeskType == Template {
		saveDeskButton = nodewith.ClassName("SaveDeskTemplateButton").Nth(0)
		savedDeskGridView = nodewith.ClassName("SavedDeskGridView").Nth(0)
	} else if savedDeskType == SaveAndRecall {
		saveDeskButton = nodewith.ClassName("SaveDeskTemplateButton").Nth(1)
		savedDeskGridView = nodewith.ClassName("SavedDeskGridView").Nth(1)
	} else {
		return errors.New("unknown savedDeskType, must be `kTemplate' or 'kSaveAndRecall'")
	}

	// Save a desk.
	if err := uiauto.Combine(
		"save a desk",
		ac.LeftClick(saveDeskButton),
		// Wait for the saved desk grid to show up.
		ac.WaitUntilExists(savedDeskGridView),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to save a desk")
	}

	// Type savedDeskName and press "Enter".
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create keyboard")
	}
	defer kb.Close()
	if err := kb.Type(ctx, savedDeskName); err != nil {
		return errors.Wrapf(err, "cannot type %q: ", savedDeskName)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "cannot press 'Enter'")
	}

	return nil
}

// WaitForSavedDeskSync waits 30 seconds for the saved desk sync.
// This assumes overview grid is live now.
// TODO(yongshun): Check if in overview mode and error out early if not.
func WaitForSavedDeskSync(ctx context.Context, ac *uiauto.Context) {
	libraryButton := nodewith.Name("Library")
	ac.WithTimeout(30 * time.Second).WaitUntilExists(libraryButton)(ctx)
}

// IsLibraryButtonVisible returns if the library button is visible.
// This assumes overview grid is live now.
// TODO(yongshun): Check if in overview mode and error out early if not.
func IsLibraryButtonVisible(ctx context.Context, ac *uiauto.Context) (bool, error) {
	libraryButton := nodewith.Name("Library")

	libraryButtonInfo, err := ac.NodesInfo(ctx, libraryButton)
	if err != nil {
		return false, errors.Wrap(err, "failed to try to find the library button")
	}

	return len(libraryButtonInfo) != 0, nil
}

// EnterLibraryPage enters the library page from desk bar.
// This assumes overview grid is live now.
// TODO(yongshun): Check if in overview mode and error out early if not.
func EnterLibraryPage(ctx context.Context, ac *uiauto.Context) error {
	libraryButton := nodewith.ClassName("ZeroStateIconButton").Name("Library")
	savedDeskGridView := nodewith.ClassName("SavedDeskGridView").Nth(0)

	var visible bool
	var err error
	if visible, err = IsLibraryButtonVisible(ctx, ac); err != nil {
		return errors.Wrap(err, "failed to check if library button is visible")
	}
	if visible == false {
		return errors.Wrap(err, "library button is not visible")
	}

	// Show the saved desk grid.
	if err = uiauto.Combine(
		"show the saved desk grid",
		ac.LeftClick(libraryButton),
		// Wait for the saved desk grid to show up.
		ac.WaitUntilExists(savedDeskGridView),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to show the saved desk grid")
	}

	return nil
}

// FindSavedDesks returns a list of saved desk nodes.
// This assumes library page is live now.
func FindSavedDesks(ctx context.Context, ac *uiauto.Context) ([]uiauto.NodeInfo, error) {
	savedDeskItemView := nodewith.ClassName("SavedDeskItemView")
	savedDeskItemViewInfo, err := ac.NodesInfo(ctx, savedDeskItemView)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find SavedDeskItemView")
	}
	return savedDeskItemViewInfo, nil
}

// LaunchSavedDesk verifies the existence of a saved desk then launches the saved desk of index.
// This assumes library page is live now.
func LaunchSavedDesk(ctx context.Context, ac *uiauto.Context, savedDeskName string, index int) error {
	savedDesk := nodewith.ClassName("SavedDeskItemView").Nth(index)
	savedDeskNameView := nodewith.ClassName("SavedDeskNameView").Name(savedDeskName)
	savedDeskMiniView :=
		nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", savedDeskName))

	// Launch the saved desk.
	if err := uiauto.Combine(
		"launch the saved desk",
		// Verify the existence of the saved desk.
		ac.WaitUntilExists(savedDeskNameView),
		ac.LeftClick(savedDesk),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch a saved desk")
	}

	// Press enter.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "cannot press 'Enter'")
	}

	// Wait for the new desk to appear.
	if err := uiauto.Combine(
		"wait for the saved desk",
		ac.WaitUntilExists(savedDeskMiniView),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch a saved desk")
	}

	return nil
}

// VerifySavedDesk verifies that the saved desks are expected as the given savedDeskNames.
// This assumes library page is live now.
func VerifySavedDesk(ctx context.Context, ac *uiauto.Context, savedDeskNames []string) error {
	savedDeskNameView := nodewith.ClassName("SavedDeskNameView")

	// Verify the saved desk count and name.
	savedDeskNameViewInfo, err := ac.NodesInfo(ctx, savedDeskNameView)
	if err != nil {
		return errors.Wrap(err, "failed to find SavedDeskNameView")
	}
	if len(savedDeskNameViewInfo) != len(savedDeskNames) {
		return errors.Wrapf(err, "found inconsistent number of saved desk(s): got %v, want %v", len(savedDeskNameViewInfo), len(savedDeskNames))
	}
	for i, info := range savedDeskNameViewInfo {
		if info.Name != savedDeskNames[i] {
			return errors.Wrapf(err, "found inconsistent saved desk name at index %v: got %s, want %s", i, info.Name, savedDeskNames[i])
		}
	}

	return nil
}

// DeleteAllSavedDesks cleans up saved desks by using a series of mouse and keyboard events to delete each desk.
// This assumes library page is live now.
func DeleteAllSavedDesks(ctx context.Context, ac *uiauto.Context, tconn *chrome.TestConn) error {
	savedDesk := nodewith.ClassName("SavedDeskItemView")
	closeButton := nodewith.ClassName("CloseButton").Name("Delete")
	deleteDialog := nodewith.ClassName("SavedDeskDialog")

	// Define keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed create keyboard")
	}
	defer kb.Close()

	// Find all saved desks.
	savedDeskInfo, err := FindSavedDesks(ctx, ac)
	if err != nil {
		return errors.Wrap(err, "failed to find saved desks")
	}

	// Delete all saved desks one by one.
	for i := range savedDeskInfo {
		firstSavedDesk := savedDesk.First()
		if err := ac.MouseMoveTo(firstSavedDesk, 0)(ctx); err != nil {
			return errors.Wrapf(err, "failed to mouse over to saved desk at position %v", i+1)
		}
		if err := uiauto.Combine(
			"Delete saved desks",
			ac.WaitUntilExists(closeButton),
			ac.LeftClick(closeButton),
			ac.WaitUntilExists(deleteDialog),
			kb.AccelAction("Enter"),
		)(ctx); err != nil {
			return errors.Wrapf(err, "fail to delete saved desk at position %v", i+1)
		}
	}

	return nil
}
