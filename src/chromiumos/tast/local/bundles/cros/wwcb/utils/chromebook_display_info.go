// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// GetInternalAndExternalDisplays returns internal and external display info.
func GetInternalAndExternalDisplays(ctx context.Context, tconn *chrome.TestConn) (result DisplayLayout, err error) {
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return result, err
	}

	var foundInt, foundExt bool
	for _, info := range infos {
		if info.IsInternal {
			result.Internal = info
			foundInt = true
		} else if !foundExt {
			// Get the first external display info.
			result.External = info
			foundExt = true
		}
	}

	if !foundInt || !foundExt {
		err = errors.Wrap(err, "not enough displays: need at least one internal display and one external display")
		return result, err
	}

	return result, err
}

// EnsureDisplayPrimary checks whether the given display is in requested display property. If not, make sure to set display property to the requested display property.
func EnsureDisplayPrimary(ctx context.Context, tconn *chrome.TestConn, disp *display.Info) error {
	if disp.IsPrimary {
		return nil
	}

	testing.ContextLogf(ctx, "Setting display [%s,%s] to be primary", disp.ID, disp.Name)

	// Set the display to primary.
	isPrimary := true
	if err := display.SetDisplayProperties(ctx, tconn, disp.ID, display.DisplayProperties{IsPrimary: &isPrimary}); err != nil {
		return errors.Wrap(err, "failed to set display properties")
	}

	// Expect the display is primary. Return err after poll timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get primary display info")
		}
		if primaryInfo.ID != disp.ID {
			return errors.New("unable to set display as primary")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

// SetMirrorDisplay sets the mirror display settings.
func SetMirrorDisplay(ctx context.Context, tconn *chrome.TestConn, want checked.Checked) error {
	ui := uiauto.New(tconn)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
	if err != nil {
		return errors.Wrap(err, "failed to launch os-settings Device page")
	}

	displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if err := ui.LeftClickUntil(displayFinder, ui.WithTimeout(3*time.Second).WaitUntilGone(displayFinder))(ctx); err != nil {
		return errors.Wrap(err, "failed to launch display page")
	}

	mirrorFinder := nodewith.Name("Mirror Built-in display").Role(role.CheckBox).Ancestor(ossettings.WindowFinder)
	// Find the node info for the mirror checkbox.
	nodeInfo, err := ui.Info(ctx, mirrorFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}
	if nodeInfo.Checked != want {
		testing.ContextLog(ctx, "Click 'Mirror Built-in display' checkbox")
		if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}
	}

	return settings.Close(ctx)
}
