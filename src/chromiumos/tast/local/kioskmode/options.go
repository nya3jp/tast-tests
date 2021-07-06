// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kioskmode

import (
	"chromiumos/tast/common/policy"
)

// ExtraPolicies adds extra policies to be applied with Kiosk app.
func ExtraPolicies(p []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraPolicies = p
		return nil
	}
}

// LoadSigninProfileExtension sets the key for loading test extension on Signin
// screen.
func LoadSigninProfileExtension(k string) Option {
	return func(cfg *MutableConfig) error {
		cfg.SigninExtKey = &k
		return nil
	}
}

// DefaultLocalAccounts uses default Kiosk applications configuration stored
// in kioskmode.defaultLocalAccountsConfiguration.
func DefaultLocalAccounts() Option {
	return func(cfg *MutableConfig) error {
		cfg.DeviceLocalAccounts = &defaultLocalAccountsConfiguration
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
func PublicAccountPolicies(accountID string, p []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		if cfg.PublicAccountPolicies == nil {
			cfg.PublicAccountPolicies = make(map[string][]policy.Policy)
		}
		cfg.PublicAccountPolicies[accountID] = p
		return nil
	}
}
