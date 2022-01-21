// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RenameDesk,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that reordering desk by drag & drop and keyboard shortcuts works well",
		Contacts: []string{
			"zxdan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// RenameDesk tests the behaviors of renaming desks.
func RenameDesk(ctx context.Context, s *testing.State) {
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
	defer kb.Close()

	// Test 1: Change the first desk'name. By clicking the default desk button.
	zeroStateDefaultDeskButton := nodewith.ClassName("ZeroStateDefaultDeskButton")
	desk1NameView := nodewith.ClassName("DeskNameView").Name("Desk 1")
	desk1Name := "Cat"
	if err := uiauto.Combine(
		"change the first desk's name",
		ui.LeftClick(zeroStateDefaultDeskButton),
		// The focus on the new desk should be on the desk name field.
		ui.WaitUntilExists(desk1NameView.Focused()),
		kb.TypeAction(desk1Name),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to change the name of the first desk: ", err)
	}

	if err := checkDeskName(ctx, tconn, 0, desk1Name); err != nil {
		s.Fatal("Failed to check the first desk name: ", err)
	}

	// Test 2: Add a new desk. The corresponding desk name view should be focused and can be edited.
	zeroAddDeskButton := nodewith.ClassName("ZeroStateIconButton")
	desk2NameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	desk2Name := "Dog"
	if err := uiauto.Combine(
		"create a new desk by clicking add desk button",
		ui.LeftClick(zeroAddDeskButton),
		// The focus on the new desk should be on the desk name field.
		ui.WaitUntilExists(desk2NameView.Focused()),
		kb.TypeAction(desk2Name),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to change the name of the second desk: ", err)
	}

	if err := checkDeskName(ctx, tconn, 1, desk2Name); err != nil {
		s.Fatal("Failed to check the second desk name: ", err)
	}

	// Test 3: Add a new desk. The desk name can be edited by clicking the desk name view.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create the third desk: ", err)
	}

	desk3NameView := nodewith.ClassName("DeskNameView").Name("Desk 3")
	desk3Name := "Monkey"

	if err := changeDeskNameByClickingNameView(ctx, tconn, desk3NameView, desk3Name); err != nil {
		s.Fatal("Failed to change the third desk name: ", err)
	}
	if err := checkDeskName(ctx, tconn, 2, desk3Name); err != nil {
		s.Fatal("Failed to check the third desk name: ", err)
	}

	desk3NameView = nodewith.ClassName("DeskNameView").Name("Monkey")
	// Test 4: The change of the desk name will be discarded by hitting escape.
	if err := uiauto.Combine(
		"discard change by hitting escape",
		ui.LeftClick(desk3NameView),
		ui.WaitUntilExists(desk3NameView.Focused()),
		kb.TypeAction("Random Name"),
		kb.AccelAction("Esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to discard the change of the third desk name: ", err)
	}

	if err := checkDeskName(ctx, tconn, 2, desk3Name); err != nil {
		s.Fatal("Failed to check the third desk name: ", err)
	}

	// Test 5: Repeated name is allowed.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create the fourth desk: ", err)
	}

	desk4NameView := nodewith.ClassName("DeskNameView").Name("Desk 4")
	if err := changeDeskNameByClickingNameView(ctx, tconn, desk4NameView, desk2Name); err != nil {
		s.Fatal("Failed to change the fourth desk name: ", err)
	}
	// Now, there should be two desks named Dog.
	desksNamedDog, err := ui.NodesInfo(ctx, nodewith.ClassName("DeskNameView").Name(desk2Name))
	if err != nil {
		s.Fatal("Failed to get desks named Dog: ", err)
	}

	numDesksNamedDog := len(desksNamedDog)
	if numDesksNamedDog != 2 {
		s.Fatalf("Got %d desks named Dog, but 2 desks wanted", numDesksNamedDog)
	}

	// Test 6: update default desk name when the desk order changes.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create the fifth desk: ", err)
	}

	desksMiniViewInfo, err := ash.FindDeskMiniViews(ctx, ui)
	if err != nil {
		s.Fatal("Failed to get desks mini view info: ", err)
	}

	desk4Loc := desksMiniViewInfo[3].Location
	desk5Loc := desksMiniViewInfo[4].Location

	// Swap the position of desk 4 and desk 5.
	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	if err := pc.Drag(
		desk4Loc.CenterPoint(),
		pc.DragTo(desk5Loc.CenterPoint(), 3*time.Second))(ctx); err != nil {
		s.Fatal("Failed to swap desk 4 and desk 5: ", err)
	}

	// The "Desk 5" should be updated to "Desk 4" after reordering.
	if err := checkDeskName(ctx, tconn, 3, "Desk 4"); err != nil {
		s.Fatal("Failed to get updated default desk name: ", err)
	}

}

// updateDeskNodesInfo updates the desks nodes by exiting and re-entering overview mode.
func updateDeskNodesInfo(ctx context.Context, tconn *chrome.TestConn) error {
	// Here, we need to do some operations to get the name of desk nodes updated.
	// Otherwise, we will still get the stable desk name.

	// Exit Overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to exit the Overview")
	}
	// Re-enter Overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enter the Overview")
	}

	return nil
}

// checkDeskName checks if the desk at id position has the same name as given expectedName.
func checkDeskName(ctx context.Context, tconn *chrome.TestConn, id int, expectedName string) error {
	ui := uiauto.New(tconn)

	// updates the desk nodes info before checking.
	updateDeskNodesInfo(ctx, tconn)

	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, ui)
	if err != nil {
		return errors.Wrap(err, "failed to get desk mini views info")
	}

	fullExpectedName := fmt.Sprintf("Desk: %s", expectedName)

	deskNum := len(deskMiniViewsInfo)

	// If there is no desk mini view, check the name of the default desk button.
	if deskNum == 0 && id == 0 {
		zeroStateDefaultDeskButton := nodewith.ClassName("ZeroStateDefaultDeskButton")
		defaultDeskInfo, err := ui.Info(ctx, zeroStateDefaultDeskButton)
		if err != nil {
			return errors.Wrap(err, "failed to get default desk button info")
		}

		if defaultDeskInfo.Name != fullExpectedName {
			return errors.Errorf("desk %d name: %s is not as expected name: %s", id, defaultDeskInfo.Name, fullExpectedName)
		}

		return nil
	}

	if deskNum <= id {
		return errors.Errorf("desk id %d beyonds total desk number %d", id, deskNum)
	}

	deskName := deskMiniViewsInfo[id].Name
	if deskName != fullExpectedName {
		return errors.Errorf("desk %d name: %s is not as expected name: %s", id, deskName, fullExpectedName)
	}

	return nil
}

// changeDeskNameByClickingNameView changes a desk name to newName by clicking the deskNameView.
func changeDeskNameByClickingNameView(ctx context.Context, tconn *chrome.TestConn, deskNameView *nodewith.Finder, newName string) error {
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a keyboard")
	}
	defer kb.Close()

	if err := uiauto.Combine(
		"change a desk name by clicking its desk name view",
		ui.LeftClick(deskNameView),
		ui.WaitUntilExists(deskNameView.Focused()),
		kb.TypeAction(newName),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to change the name of the desk")
	}

	return nil
}
