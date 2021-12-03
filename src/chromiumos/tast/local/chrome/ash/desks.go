// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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

// FindDeskTemplates returns a list of desk template nodes.
func FindDeskTemplates(ctx context.Context, ac *uiauto.Context) ([]uiauto.NodeInfo, error) {
	desksTemplatesItemView := nodewith.ClassName("DesksTemplatesItemView")
	desksTemplatesItemViewInfo, err := ac.NodesInfo(ctx, desksTemplatesItemView)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find DesksTemplatesItemView")
	}
	return desksTemplatesItemViewInfo, nil
}
