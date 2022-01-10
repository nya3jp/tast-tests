// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"chromiumos/tast/common/policy"
)

const (
	// These extensions are unlisted on the Chrome Web Store but can be
	// downloaded directly using the extension IDs.
	// The code for the extensions can be found in the Chromium repo at
	// chrome/test/data/extensions/api_test/login_screen_apis/.

	// LoginScreenExtensionID is the ID for "Login screen APIs test extension".
	LoginScreenExtensionID = "oclffehlkdgibkainkilopaalpdobkan"
	// InSessionExtensionID is the ID for "Login screen APIs in-session test extension".
	InSessionExtensionID = "ofcpkomnogjenhfajfjadjmjppbegnad"
)

// DefaultPolicies returns the default polices to enable managed guest session.
func DefaultPolicies(accountID string) []policy.Policy {

	accountType := policy.AccountTypePublicSession

	return []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &accountID,
					AccountType: &accountType,
				},
			},
		},
		&policy.DeviceLoginScreenExtensions{
			Val: []string{LoginScreenExtensionID},
		},
	}
}
