// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
)

// DeskMiniViewBoundsInfo corresponds to the type by the same name in
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl
type DeskMiniViewBoundsInfo struct {
	Success bool        `json:"success"`
	Bounds  coords.Rect `json:"bounds"`
}

// CreateNewDesk requests Ash to create a new Virtual Desk which would fail if
// the maximum number of desks have been reached.
func CreateNewDesk(ctx context.Context, tconn *chrome.TestConn) error {
	expr := `tast.promisify(chrome.autotestPrivate.createNewDesk)()`
	success := false
	if err := tconn.EvalPromise(ctx, expr, &success); err != nil {
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
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.activateDeskAtIndex)(%v)`, index)
	success := false
	if err := tconn.EvalPromise(ctx, expr, &success); err != nil {
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
	expr := `tast.promisify(chrome.autotestPrivate.removeActiveDesk)()`
	success := false
	if err := tconn.EvalPromise(ctx, expr, &success); err != nil {
		return err
	}
	if !success {
		return errors.New("failed to remove the active desk")
	}
	return nil
}

// GetDeskMiniViewBounds gets the bounds of a desk mini-view, in screen coordinates.
// This call will fail if the desks bar is not showing or the arguments are not all
// valid.
func GetDeskMiniViewBounds(ctx context.Context, tconn *chrome.TestConn, displayID int, index int) (*coords.Rect, error) {
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.getDeskMiniViewBounds)(%v, %v)`, displayID, index)
	var info DeskMiniViewBoundsInfo
	if err := tconn.EvalPromise(ctx, expr, &info); err != nil {
		return err, nil
	}
	if !info.Success {
		return errors.Errorf("failed to get desk mini-view bounds on display %v at index %v", displayID, index), nil
	}
	return nil, &info.Bounds
}
