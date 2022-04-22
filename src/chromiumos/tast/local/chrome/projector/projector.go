// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// RefreshApp returns an action that refreshes the Screencast app by right-clicking.
func RefreshApp(ctx context.Context, tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	appWindow := nodewith.Name("Screencast").Role(role.Application)
	reload := nodewith.Name("Reload Ctrl+R").Role(role.MenuItem)

	return func(ctx context.Context) error {
		if err := uiauto.Combine("refresh app through right-click context menu",
			ui.RightClickUntil(appWindow, ui.Exists(reload)),
			ui.LeftClick(reload),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to refresh app")
		}
		return nil
	}
}

// DismissOnboardingDialog closes the onboarding dialog if it exists.
func DismissOnboardingDialog(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	closeOnboardingButton := nodewith.Name("Skip tour").Role(role.Button)

	// Since each user only sees the onboarding flow a maximum of three
	// times, the onboarding dialog may not appear.
	if err := ui.WaitUntilExists(closeOnboardingButton)(ctx); err != nil {
		// Onboarding dialog not found.
		return nil
	}

	testing.ContextLog(ctx, "Dismissing the onboarding dialog")
	if err := ui.LeftClickUntil(closeOnboardingButton, ui.Gone(closeOnboardingButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to close the onboarding dialog")
	}
	return nil
}
