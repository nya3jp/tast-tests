// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mgs provides ways to set policies for local device accounts
// in MGS mode.
package mgs

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

var (
	// MgsAccountID is the default MGS ID.
	MgsAccountID = "defaultMgsSetByTast"
	// AccountType is the default public session account type.
	AccountType = policy.AccountTypePublicSession

	// Default MGS account configuration.
	mgsAccountPolicy = policy.DeviceLocalAccountInfo{
		AccountID:   &MgsAccountID,
		AccountType: &AccountType,
	}

	// Default device local account configuration enclosing MGS account.
	accountsConfiguration = policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			mgsAccountPolicy,
		},
	}
)

// mgs holds chrome and fakedms instances.
type mgs struct {
	cr   *chrome.Chrome
	fdms *fakedms.FakeDMS
}

// Close closes chrome, cleans and refreshes empty policies.
func (m *mgs) Close(ctx context.Context) (retErr error) {
	// Apply empty policies.
	if err := policyutil.ServeAndRefresh(ctx, m.fdms, m.cr, []policy.Policy{}); err != nil {
		retErr = errors.Wrap(err, "failed to apply empty policies")
	}
	if err := m.cr.Close(ctx); err != nil {
		if retErr != nil {
			testing.ContextLog(ctx, "Failed to close chrome: ", retErr)
		}
		retErr = errors.Wrap(err, "failed to close chrome")
	}
	return retErr
}

// New starts Chrome, sets passed MGS related options to policies and
// restarts Chrome. Use mgs.AutoLaunch()to auto start MGS for
// a given account ID. Alternatively use mgs.ExtraChromeOptions()
// passing chrome.LoadSigninProfileExtension(). In that case Chrome is started
// and stays on Signin screen with mgs accounts loaded.
// Use defer mgs.Close() to perform clean up including closing Chrome instance.
func New(ctx context.Context, fdms *fakedms.FakeDMS, opts ...Option) (*mgs, *chrome.Chrome, error) {
	cfg, err := NewConfig(opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to process options")
	}

	if cfg.m.MGSAccounts == nil {
		return nil, nil, errors.Wrap(err, "mgs accounts were not set")
	}

	err = func(ctx context.Context) error {
		testing.ContextLog(ctx, "MGS: starting Chrome to set MGS policies")
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
			cfg.m.MGSAccounts,
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
		pb.AddPolicies(policies)
		// Handle public account policies.
		if cfg.m.PublicAccountPolicies != nil {
			for accountID, policies := range cfg.m.PublicAccountPolicies {
				pb.AddPublicAccountPolicies(accountID, policies)
			}
		}

		// Update policies.
		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			return errors.Wrap(err, "failed to serve and refresh policies")
		}

		// Close the previous Chrome instance.
		defer cr.Close(ctx)
		return nil
	}(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed preparing Chrome to start with MGS")
	}

	var cr *chrome.Chrome
	crOpts := []chrome.Option{
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	}

	if cfg.m.AutoLaunch {
		crOpts = append(crOpts, chrome.NoLogin())
		testing.ContextLog(ctx, "MGS: starting MGS in auto launch mode")
	} else {
		crOpts = append(crOpts, chrome.DeferLogin())
		testing.ContextLog(ctx, "MGS: starting Chrome with MGS accounts loaded")
	}

	crOpts = append(crOpts, cfg.m.ExtraChromeOptions...)

	// Restart Chrome.
	cr, err = chrome.New(ctx, crOpts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Chrome restart failed")
	}

	return &mgs{cr: cr, fdms: fdms}, cr, nil
}
