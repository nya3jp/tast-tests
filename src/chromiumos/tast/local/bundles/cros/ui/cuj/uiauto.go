// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// UnsetMirrorDisplay unsets the mirror display settings.
func UnsetMirrorDisplay(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	testing.ContextLog(ctx, "Launch os-settings to disable mirror")
	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
	if err != nil {
		return errors.Wrap(err, "failed to launch os-settings Device page")
	}

	displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if err := ui.LeftClick(displayFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch display page")
	}

	mirrorFinder := nodewith.Name("Mirror Built-in display").Role(role.CheckBox).Ancestor(ossettings.WindowFinder)
	// Find the node info for the mirror checkbox
	nodeInfo, err := ui.Info(ctx, mirrorFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}
	if nodeInfo.Checked == "true" {
		testing.ContextLog(ctx, "Click 'Mirror Built-in display' checkbox")
		if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}
	}

	if err := settings.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close settings")
	}
	return nil
}
