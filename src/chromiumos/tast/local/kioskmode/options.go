// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kioskmode

import (
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
)

// ExtraPolicies adds extra policies to be applied with Kiosk app.
func ExtraPolicies(p []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraPolicies = p
		return nil
	}
}

// DefaultLocalAccounts uses default Kiosk applications configuration stored
// in kioskmode.defaultLocalAccountsConfiguration.
func DefaultLocalAccounts() Option {
	return func(cfg *MutableConfig) error {
		cfg.DeviceLocalAccounts = &DefaultLocalAccountsConfiguration
		return nil
	}
}

// CustomLocalAccounts sets custom local accounts on DUT. Use when the default
// configuration provided by DefaultLocalAccounts() option is not enough.
// E.g. when test has to use a specific website or a specific Chrome App.
func CustomLocalAccounts(accounts *policy.DeviceLocalAccounts) Option {
	return func(cfg *MutableConfig) error {
		cfg.DeviceLocalAccounts = accounts
		return nil
	}
}

// AutoLaunch sets Kiosk app ID to be started after Chrome restart. When used
// then defer cleaning and refreshing policies policyutil.ServeAndRefresh(ctx,
// fdms, cr, []policy.Policy{}). Otherwise with next Chrome restart Kiosk will
// auto start.
func AutoLaunch(accountID string) Option {
	return func(cfg *MutableConfig) error {
		cfg.AutoLaunch = true
		cfg.AutoLaunchKioskAppID = &accountID
		return nil
	}
}

// PublicAccountPolicies adds policies that will be applied to the account.
func PublicAccountPolicies(accountID string, policies []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		if cfg.PublicAccountPolicies == nil {
			cfg.PublicAccountPolicies = make(map[string][]policy.Policy)
		}
		cfg.PublicAccountPolicies[accountID] = append(cfg.PublicAccountPolicies[accountID], policies...)
		return nil
	}
}

// ExtraChromeOptions passes Chrome options that will be applied to the Chrome
// instance running in Kiosk mode.
func ExtraChromeOptions(opts ...chrome.Option) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraChromeOptions = opts
		return nil
	}
}
