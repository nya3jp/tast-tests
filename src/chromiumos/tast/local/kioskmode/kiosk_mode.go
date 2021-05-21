// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kioskmode provides ways to set policies for local device accounts
// in a Kiosk mode.
package kioskmode

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
)

const (
	kioskStarting        = "Starting kiosk mode"
	kioskLaunchSucceeded = "Kiosk launch succeeded"

	logScanTimeout = 60 * time.Second
)

// DeviceLocalAccountInfo uses *string instead of string for internal data
// structure. That is needed since fields in json are marked as omitempty.
var (
	WebKioskAccountID   = "arbitrary_id_web_kiosk_1"
	webKioskAccountType = policy.AccountTypeKioskWebApp
	webKioskIconURL     = "https://www.google.com"
	webKioskTitle       = "TastKioskModeSetByPolicyGooglePage"
	webKioskURL         = "https://www.google.com"

	KioskAppAccountID   = "arbitrary_id_store_app_2"
	kioskAppAccountType = policy.AccountTypeKioskApp
	kioskAppID          = "aajgmlihcokkalfjbangebcffdoanjfo"
	KioskAppBtnNode     = nodewith.Name("Simple Printest").ClassName("MenuItemView")
)

// ServerPoliciesForDefaultApplications serves DeviceLocalAccounts policy with
// default configuration for Kiosk accounts and triggers refresh of policies.
// If you want to see the effect - restart Chrome instance to see Apps button
// on the sign-in screen.
func ServerPoliciesForDefaultApplications(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	// ServerAndRefresh is used instead of ServerAndVerify since Verify part
	// uses Autotest private api that returns only Enterprise policies values
	// but not device policies values.
	return policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &KioskAppAccountID,
					AccountType: &kioskAppAccountType,
					KioskAppInfo: &policy.KioskAppInfo{
						AppId: &kioskAppID,
					},
				},
				{
					AccountID:   &WebKioskAccountID,
					AccountType: &webKioskAccountType,
					WebKioskAppInfo: &policy.WebKioskAppInfo{
						Url:     &webKioskURL,
						Title:   &webKioskTitle,
						IconUrl: &webKioskIconURL,
					},
				},
			},
		},
	})
}

// CheckLogsConfirmingKioskStartedSucceesfully uses reader for looking for logs
// that confirm Kiosk mode starting and also successful launch of Kiosk.
// r Reader instance should be processing all logs or filtered for Chrome only.
func CheckLogsConfirmingKioskStartedSucceesfully(ctx context.Context, r *syslog.Reader) error {
	// Check that Kiosk starts successfully.
	if _, err := r.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskStarting)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify starting of Kiosk mode")
	}

	if _, err := r.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceeded)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify successful launch of Kiosk mode")
	}
	return nil
}
