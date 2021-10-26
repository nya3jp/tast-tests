// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mgs provides ways to set policies for local device accounts
// in MGS mode.
package mgs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

var (
	mgsAccountID = "foo@bar.com"
	accountType  = policy.AccountTypePublicSession

	// These extensions are unlisted on the Chrome Web Store but can be
	// downloaded directly using the extension IDs.
	// The code for the extensions can be found in the Chromium repo at
	// chrome/test/data/extensions/api_test/login_screen_apis/.
	// ID for "Login screen APIs test extension".
	loginScreenExtensionID = "oclffehlkdgibkainkilopaalpdobkan"
	// ID for "Login screen APIs in-session test extension".
	inSessionExtensionID = "ofcpkomnogjenhfajfjadjmjppbegnad"

	// MGS account configuration.
	mgsAccountPolicy = policy.DeviceLocalAccountInfo{
		AccountID:   &mgsAccountID,
		AccountType: &accountType,
	}

	// Device local account configuration enclosing MGS account.
	accountsConfiguration = policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			mgsAccountPolicy,
		},
	}
)

type mgs struct {
	cr *chrome.Chrome
}

func (m *mgs) Close(ctx context.Context) {
	// apply empty policies
	m.cr.Close(ctx)
}

// ConfirmMGSStarted uses reader for looking for logs that confirm MGS mode
// starting and also successful launch of MGS.
// reader Reader instance should be processing all logs filtered for Chrome
// only.
func ConfirmMGSStarted(ctx context.Context, reader *syslog.Reader) error {
	// Check that MGS starts successfully.
	testing.ContextLog(ctx, "Waiting for MGS mode start")

	const (
		mgsStarting        = "Starting MGS mode"
		mgsLaunchSucceeded = "MGS launch succeeded"

		// logScanTimeout is a timeout for log messages indicating MGS startup
		// and successful launch to be present. It is set to over a minute as MGS
		// mode launch varies depending on device. crbug.com/1222136
		logScanTimeout = 90 * time.Second
	)

	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, mgsStarting)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify starting of MGS mode")
	}

	testing.ContextLog(ctx, "Waiting for successful MGS mode launch")
	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, mgsLaunchSucceeded)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify successful launch of MGS mode")
	}
	return nil
}

// New starts Chrome, sets passed MGS related options to policies and
// restarts Chrome. When mgs.AutoLaunch() is used, then it auto starts
// given MGS account id. Alternatively use mgs.ExtraChromeOptions()
// passing chrome.LoadSigninProfileExtension(). In that case Chrome is started
// and stays on Signin screen with mgs accounts loaded.
// If mgs.AutoLaunch() option is used you should defer cleaning and
// refreshing policies policyutil.ServeAndRefresh(ctx, fdms, cr,
// []policy.Policy{}).
func New(ctx context.Context, fdms *fakedms.FakeDMS, opts ...Option) (mgs, *chrome.Chrome, error) {
	cfg, err := NewConfig(opts)
	if err != nil {
		return mgs{}, nil, errors.Wrap(err, "failed to process options")
	}

	if cfg.m.MGSDefaultAccount == nil && cfg.m.MGSAccounts == nil {
		return mgs{}, nil, errors.Wrap(err, "mgs accounts were not set")
	}

	err = func(ctx context.Context) error {
		testing.ContextLog(ctx, "mgs_mode - starting Chrome to set MGS policies")
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}), // Required as refreshing policies require test API.
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepState(),
			chrome.ExtraArgs("--force-devtools-available"),
		)
		if err != nil {
			return errors.Wrap(err, "failed to start Chrome")
		}

		// Set local accounts policy.
		policies := []policy.Policy{
			cfg.m.MGSDefaultAccount,
		}

		if cfg.m.MGSAccounts != nil {
			policies = append(policies, cfg.m.MGSAccounts)
		}

		// Handle the AutoLaunch setup.
		if cfg.m.AutoLaunch == true {
			policies = append(policies, &policy.DeviceLocalAccountAutoLoginId{
				Val: *cfg.m.AutoLaunchMGSAppID,
			})
		}

		// Handle setting device policies.
		if cfg.m.ExtraPolicies != nil {
			policies = append(policies, cfg.m.ExtraPolicies...)
		}

		pb := fakedms.NewPolicyBlob()
		// pb.AddPolicies(mgsAccountPolicy)
		pb.AddPublicAccountPolicy(mgsAccountID, &policy.ExtensionInstallForcelist{
			Val: []string{inSessionExtensionID},
		})

		// Update policies.
		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			return errors.Wrap(err, "failed to serve and refresh policies")
		}

		// Close the previous Chrome instance.
		defer cr.Close(ctx)
		return nil
	}(ctx)
	if err != nil {
		return mgs{}, nil, errors.Wrap(err, "failed preparing Chrome to start with MGS")
	}

	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		return mgs{}, nil, errors.Wrap(err, "failed to start log reader")
	}
	defer reader.Close()

	var cr *chrome.Chrome
	if cfg.m.AutoLaunch {
		opts := []chrome.Option{
			chrome.NoLogin(),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepState(),
			chrome.ExtraArgs("--force-devtools-available"),
		}
		opts = append(opts, cfg.m.ExtraChromeOptions...)

		testing.ContextLog(ctx, "mgs - starting Chrome in MGS mode")
		// Restart Chrome. After that MGS auto starts.
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			return mgs{}, nil, errors.Wrap(err, "Chrome restart failed")
		}

		if err := ConfirmMGSStarted(ctx, reader); err != nil {
			cr.Close(ctx)
			return mgs{}, nil, errors.Wrap(err, "there was a problem while checking chrome logs for MGS related entries")
		}
	} else {
		opts := []chrome.Option{
			chrome.DeferLogin(),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepState(),
			chrome.ExtraArgs("--force-devtools-available"),
		}
		opts = append(opts, cfg.m.ExtraChromeOptions...)

		testing.ContextLog(ctx, "mgs - starting Chrome on Signin screen with MGS account")
		// Restart Chrome. Chrome stays on Sing-in screen
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			return mgs{}, nil, errors.Wrap(err, "Chrome restart failed")
		}
	}

	return mgs{cr: cr}, cr, nil
}
