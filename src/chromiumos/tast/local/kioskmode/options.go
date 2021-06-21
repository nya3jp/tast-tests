// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kioskmode

import "chromiumos/tast/common/policy"

// AddExtraPolicies adds device policies to be applied with Kiosk app.
func AddExtraPolicies(p []policy.Policy) Option {
	return func(cfg *MutableConfig) error {
		cfg.ExtraPolicies = p
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
