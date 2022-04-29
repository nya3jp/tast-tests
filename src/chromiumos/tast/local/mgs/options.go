// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
)

// DefaultAccount applies default local account configuration using one
// arbitrary MGS definition in the mgs package.
// MGS account id is exposed from the package.
func DefaultAccount() Option {
	return func(cfg *MutableConfig) error {
		cfg.MGSAccounts = &accountsConfiguration
		return nil
	}
}

// Accounts creates DeviceLocalAccountInfo (of a type policy.AccountTypePublicSession)
// for each passed accountID and adds them all to policy.DeviceLocalAccounts.
func Accounts(accountIDs ...string) Option {
	return func(cfg *MutableConfig) error {
		var mgsAccountPolicies []policy.DeviceLocalAccountInfo
		for _, accountID := range accountIDs {
			mgsPolicy := policy.DeviceLocalAccountInfo{
				AccountID:   &accountID,
				AccountType: &AccountType,
			}
			mgsAccountPolicies = append(mgsAccountPolicies, mgsPolicy)
		}
		accounts := policy.DeviceLocalAccounts{
			Val: mgsAccountPolicies,
		}
		cfg.MGSAccounts = &accounts
		return nil
	}
}

// ExtraPolicies adds policies to be applied with MGS.
func ExtraPolicies(p []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraPolicies = p
		return nil
	}
}

// AddPublicAccountPolicies adds policies that will be applied to the account.
func AddPublicAccountPolicies(accountID string, policies []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		if cfg.PublicAccountPolicies == nil {
			cfg.PublicAccountPolicies = make(map[string][]policy.Policy)
		}
		cfg.PublicAccountPolicies[accountID] = append(cfg.PublicAccountPolicies[accountID], policies...)
		return nil
	}
}

// AutoLaunch sets MGS ID to be started after Chrome restart.
func AutoLaunch(accountID string) Option {
	return func(cfg *MutableConfig) error {
		cfg.AutoLaunch = true
		cfg.AutoLaunchMGSAppID = &accountID
		return nil
	}
}

// ExtraChromeOptions passes Chrome options that will be applied to the Chrome
// instance running in MGS mode.
func ExtraChromeOptions(opts ...chrome.Option) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraChromeOptions = opts
		return nil
	}
}

// ExternalPolicyBlob allows to specify a policy blob that is constructed externally.
func ExternalPolicyBlob(blob *policy.Blob) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExternalPolicyBlob = blob
		return nil
	}
}
