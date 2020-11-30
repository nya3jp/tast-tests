// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// CheckNodeAttributes returns whether this node matches the given expected params.
// It will return an error with the first non-matching attribute, nil otherwise.
func CheckNodeAttributes(n *ui.Node, expectedParams ui.FindParams) error {
	if expectedRestriction, exist := expectedParams.Attributes["restriction"]; exist {
		if n.Restriction != expectedRestriction {
			return errors.Errorf("unexpected restriction: got %#v; want %#v", n.Restriction, expectedRestriction)
		}
	}
	if expectedChecked, exist := expectedParams.Attributes["checked"]; exist {
		if n.Checked != expectedChecked {
			return errors.Errorf("unexpected checked: got %#v; want %#v", n.Checked, expectedChecked)
		}
	}
	if expectedName, exist := expectedParams.Attributes["name"]; exist {
		if n.Name != expectedName {
			return errors.Errorf("unexpected name: got %#v; want %#v", n.Name, expectedName)
		}
	}
	return nil
}

// FindSettingsNode returns whether a node is found with the given params.
// It will also compare the attributes of the found node with the given expected params.
func FindSettingsNode(ctx context.Context, tconn *chrome.TestConn, params, expectedParams ui.FindParams) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "finding the node failed")
	}
	defer node.Release(ctx)
	return CheckNodeAttributes(node, expectedParams)
}

// FindPageSettingsNode returns whether a node located in the given settings page is found with the given params.
// It will also compare the attributes of the found node with the given expected params.
func FindPageSettingsNode(ctx context.Context, cr *chrome.Chrome, settingsPage string, tconn *chrome.TestConn, params, expectedParams ui.FindParams) error {
	conn, err := cr.NewConn(ctx, settingsPage)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the settings page")
	}
	defer conn.Close()
	return FindSettingsNode(ctx, tconn, params, expectedParams)
}
