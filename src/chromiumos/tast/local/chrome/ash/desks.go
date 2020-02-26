// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

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
