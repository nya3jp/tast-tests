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
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
)

const (
	kioskStarting        = "Starting kiosk mode"
	kioskLaunchSucceeded = "Kiosk launch succeeded"

	logScanTimeout = 60 * time.Second
)

var (
	// WebKioskAccountID identifier of the web Kiosk application.
	WebKioskAccountID   = "arbitrary_id_web_kiosk_1"
	webKioskAccountType = policy.AccountTypeKioskWebApp
	webKioskIconURL     = "https://www.google.com"
	webKioskTitle       = "TastKioskModeSetByPolicyGooglePage"
	webKioskURL         = "https://www.google.com"
	// DeviceLocalAccountInfo uses *string instead of string for internal data
	// structure. That is needed since fields in json are marked as omitempty.
	webKioskPolicy = policy.DeviceLocalAccountInfo{
		AccountID:   &WebKioskAccountID,
		AccountType: &webKioskAccountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:     &webKioskURL,
			Title:   &webKioskTitle,
			IconUrl: &webKioskIconURL,
		}}

	// KioskAppAccountID identifier of the Kiosk application.
	KioskAppAccountID   = "arbitrary_id_store_app_2"
	kioskAppAccountType = policy.AccountTypeKioskApp
	// kioskAppID pointing to the Printtest app - not listed in the WebStore.
	kioskAppID = "aajgmlihcokkalfjbangebcffdoanjfo"
	// KioskAppBtnNode node representing this application on the Apps menu on
	// the Sign-in screen.
	KioskAppBtnNode = nodewith.Name("Simple Printest").ClassName("MenuItemView")
	kioskAppPolicy  = policy.DeviceLocalAccountInfo{
		AccountID:   &KioskAppAccountID,
		AccountType: &kioskAppAccountType,
		KioskAppInfo: &policy.KioskAppInfo{
			AppId: &kioskAppID,
		}}

	// defaultLocalAccountsConfiguration holds default Kiosks accounts
	// configuration. Each, when setting public account policies can be
	// referred by id: KioskAppAccountID and WebKioskAccountID
	defaultLocalAccountsConfiguration = policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskAppPolicy,
			webKioskPolicy,
		},
	}
)

// SetDefaultAppPolicies serves DeviceLocalAccounts policy with default
// configuration for Kiosk accounts and triggers refresh of policies. If you
// want to see the effect - restart Chrome instance to see Apps button on the
// Sign-in screen.
// Deprecated: Use kioskmode.New(...) instead
func SetDefaultAppPolicies(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	// ServerAndRefresh is used instead of ServerAndVerify since Verify part
	// uses Autotest private api that returns only Enterprise policies values
	// but not device policies values.
	return policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{
		&defaultLocalAccountsConfiguration,
	})
}

// SetAutolaunch sets all default congifurations for Kiosk accounts and sets
// one of them to autolaunch.
// appID is the Kiosk account ID that will be autolaunched.
// Deprecated: Use kioskmode.New(...) instead
func SetAutolaunch(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, appID string) error {
	return policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{
		&defaultLocalAccountsConfiguration,
		&policy.DeviceLocalAccountAutoLoginId{
			Val: appID,
		},
	})
}

// ConfirmKioskStarted uses reader for looking for logs that confirm Kiosk mode
// starting and also successful launch of Kiosk.
// reader Reader instance should be processing all logs filtered for Chrome
// only.
func ConfirmKioskStarted(ctx context.Context, reader *syslog.Reader) error {
	// Check that Kiosk starts successfully.
	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskStarting)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify starting of Kiosk mode")
	}

	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceeded)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify successful launch of Kiosk mode")
	}
	return nil
}

// New starts Chrome, sets passed Kiosk related options to policies and
// restarts Chrome. When kioskmode.AutoLaunch() is used, then it auto starts
// given Kiosk application.
// If kioskmode.AutoLaunch() option is used you should defer cleaning and
// refreshing policies policyutil.ServeAndRefresh(ctx, fdms, cr,
// []policy.Policy{}).
func New(ctx context.Context, fdms *fakedms.FakeDMS, opts ...Option) (*chrome.Chrome, error) {
	cfg, err := NewConfig(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process options")
	}

	if cfg.m.DeviceLocalAccounts == nil {
		return nil, errors.Wrap(err, "local device accounts were not set")
	}

	func(ctx context.Context) error {
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}), // Required as refreshing policies require test API.
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment(),
		)
		if err != nil {
			return errors.Wrap(err, "failed to start Chrome")
		}

		// Set local accounts policy.
		policies := []policy.Policy{
			cfg.m.DeviceLocalAccounts,
		}

		// Handle the AutoLaunch setup.
		if cfg.m.AutoLaunch == true {
			policies = append(policies, &policy.DeviceLocalAccountAutoLoginId{
				Val: *cfg.m.AutoLaunchKioskAppID,
			})
		}

		// Handle setting device policies.
		if cfg.m.ExtraPolicies != nil {
			policies = append(policies, cfg.m.ExtraPolicies...)
		}

		// TODO: handle public account policies

		// Update policies.
		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			return errors.Wrap(err, "failed to serve and refresh policies")
		}

		// Close the previous Chrome instance.
		defer cr.Close(ctx)
		return nil
	}(ctx)

	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start log reader")
	}
	defer reader.Close()

	// Restart Chrome. After that Kiosk auto starts.
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "Chrome restart failed")
	}

	if err := ConfirmKioskStarted(ctx, reader); err != nil {
		cr.Close((ctx))
		return nil, errors.Wrap(err, "there was a problem while checking chrome logs for Kiosk related entries")
	}
	return cr, nil
}
