// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"chromiumos/tast/errors"
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
