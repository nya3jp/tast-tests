// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// ClickElement clicks on the element with the specific role and name.
// If the JavaScript fails to execute, an error is returned.
func ClickElement(ctx context.Context, tconn *chrome.TestConn, role ui.RoleType, name string) error {
	params := ui.FindParams{
		Name: name,
		Role: role,
	}

	node, err := ui.Find(ctx, tconn, params)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// WaitForElement waits for an element to exist.
// If the timeout is reached, an error is returned.
func WaitForElement(ctx context.Context, tconn *chrome.TestConn, role ui.RoleType, name string, timeout time.Duration) error {
	params := ui.FindParams{
		Name: name,
		Role: role,
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, timeout); err != nil {
		return err
	}
	return nil
}
