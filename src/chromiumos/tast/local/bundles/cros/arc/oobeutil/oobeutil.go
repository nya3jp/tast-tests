// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobeutil implements some functions used to go through OOBE screens.
// TODO(crbug.com/1327981): Use OOBE test API and move this package to `tast/local/bundles/cros/oobe` directory.
package oobeutil

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
)

// CompleteConsolidatedConsentOnboardingFlow function goes through the onboarding flow screens when the consolidated consent feature is enabled.
func CompleteConsolidatedConsentOnboardingFlow(ctx context.Context, ui *uiauto.Context) error {
	consolidatedConsentHeader := nodewith.Name("Review these terms and control your data").Role(role.Dialog)
	if err := ui.WaitUntilExists(consolidatedConsentHeader)(ctx); err != nil {
		return err
	}

	// In lower resolution screens, a `see more` button is shown and the accept button is hidden until the `see more` button is clicked.
	acceptAndContinue := nodewith.Name("Accept and continue").Role(role.Button)
	acceptButtonFound, err := ui.IsNodeFound(ctx, acceptAndContinue)
	if err != nil {
		return err
	}
	if !acceptButtonFound {
		focusedButton := nodewith.State(state.Focused, true).Role(role.Button)
		if err := ui.LeftClick(focusedButton)(ctx); err != nil {
			return err
		}
	}

	skip := nodewith.Name("Skip").Role(role.Button)
	noThanks := nodewith.Name("No thanks").Role(role.Button)
	getStarted := nodewith.Name("Get started").Role(role.Button)
	syncAccept := nodewith.NameRegex(regexp.MustCompile("Accept and continue|Got it")).Role(role.Button)
	err = uiauto.Combine("go through the oobe flow screens after the consolidated consent screen",
		ui.WaitUntilExists(acceptAndContinue),
		ui.LeftClickUntil(acceptAndContinue, ui.Gone(acceptAndContinue)),
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(syncAccept), ui.LeftClick(syncAccept)),
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(skip), ui.LeftClickUntil(skip, ui.Gone(skip))),
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(skip), ui.LeftClick(skip)),
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(noThanks), ui.LeftClickUntil(noThanks, ui.Gone(noThanks))),
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(noThanks), ui.LeftClick(noThanks)),
		ui.WaitUntilExists(getStarted),
		ui.LeftClick(getStarted),
	)(ctx)
	return err
}

// CompleteRegularOnboardingFlow function goes through the onboarding flow screens when the consolidated consent feature is disabled.
func CompleteRegularOnboardingFlow(ctx context.Context, ui *uiauto.Context, reviewArcOptions bool) error {
	skip := nodewith.Name("Skip").Role(role.StaticText)
	noThanks := nodewith.Name("No thanks").Role(role.Button)
	getStarted := nodewith.Name("Get started").Role(role.Button)
	syncAccept := nodewith.NameRegex(regexp.MustCompile("Accept and continue|Got it")).Role(role.Button)
	more := nodewith.Name("More").Role(role.Button)

	err := uiauto.Combine("go through the onboarding UI prior to ARC ToS acceptance",
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(syncAccept), ui.LeftClick(syncAccept)),
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(skip), ui.LeftClickUntil(skip, ui.Gone(skip))),
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(skip), ui.LeftClick(skip)),
		ui.WithTimeout(60*time.Second).WaitUntilExists(more),
		ui.LeftClick(more),
	)(ctx)
	if err != nil {
		return err
	}
	if reviewArcOptions {
		if err := ui.LeftClick(nodewith.Name("Review Google Play options following setup").Role(role.CheckBox))(ctx); err != nil {
			return err
		}
	}
	err = uiauto.Combine("go through the onboarding flow UI starting from ARC ToS acceptance",
		ui.LeftClick(nodewith.Name("Accept").Role(role.Button)),
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(noThanks), ui.LeftClickUntil(noThanks, ui.Gone(noThanks))),
		uiauto.IfSuccessThen(ui.WithTimeout(60*time.Second).WaitUntilExists(noThanks), ui.LeftClick(noThanks)),
		ui.WaitUntilExists(getStarted),
		ui.LeftClick(getStarted),
	)(ctx)
	return err
}

// CompleteTabletOnboarding function goes through the the tablet specific oobe screens
func CompleteTabletOnboarding(ctx context.Context, ui *uiauto.Context) error {
	next := nodewith.Name("Next").Role(role.Button)
	getStarted := nodewith.Name("Get started").Role(role.Button)
	err := uiauto.Combine("go through the tablet specific flow",
		uiauto.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClickUntil(next, ui.Gone(next))),
		uiauto.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClickUntil(next, ui.Gone(next))),
		uiauto.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(next), ui.LeftClick(next)),
		uiauto.IfSuccessThen(ui.WithTimeout(30*time.Second).WaitUntilExists(getStarted), ui.LeftClick(getStarted)),
	)(ctx)
	return err
}
