// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package thunderbolt provides Thunderbolt util functions for health tast.
package thunderbolt

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// DisableDataAccessProtection performs disabling of data access protection for peripherals through UI.
func DisableDataAccessProtection(ctx context.Context, tconn *chrome.TestConn) error {
	disableButton := nodewith.Name("Disable").Role(role.Button)
	securityPrivacy := nodewith.Name("Security and Privacy").Role(role.Link)
	dataAccessToggle := nodewith.Name("Data access protection for peripherals").Role(role.ToggleButton)

	// Launch the Settings app and wait for it to open.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch the Settings app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to appear settings app in the shelf")
	}

	cui := uiauto.New(tconn)
	if err := cui.LeftClick(securityPrivacy)(ctx); err != nil {
		return errors.Wrapf(err, "failed to left click %q with error", securityPrivacy)
	}

	info, err := cui.Info(ctx, dataAccessToggle)
	if err != nil {
		return errors.Wrap(err, "failed to get dataAccessToggle node info")
	}
	// If togglebutton already disabled we are skipping the data access disabling.
	if info.HTMLAttributes["aria-pressed"] != "false" {
		if err := cui.LeftClick(dataAccessToggle)(ctx); err != nil {
			return errors.Wrapf(err, "failed to left click %q, info %q with error", info, dataAccessToggle)
		}

		if err := cui.WaitUntilExists(disableButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for element")
		}

		if err := cui.LeftClick(disableButton)(ctx); err != nil {
			return errors.New("failed to left click disableButton")
		}
	}

	return nil
}
