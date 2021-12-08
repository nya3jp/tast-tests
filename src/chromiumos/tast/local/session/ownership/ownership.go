// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ownership provides utilities to run ownership API related tests.
package ownership

import (
	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	lm "chromiumos/system_api/login_manager_proto"
)

// BuildTestSettings returns the ChromeDeviceSettingsProto instance which
// can be used for testing settings.
func BuildTestSettings(user string) *enterprise_management_proto.ChromeDeviceSettingsProto {
	boolTrue := true
	boolFalse := false
	return &enterprise_management_proto.ChromeDeviceSettingsProto{
		GuestModeEnabled: &enterprise_management_proto.GuestModeEnabledProto{
			GuestModeEnabled: &boolFalse,
		},
		ShowUserNames: &enterprise_management_proto.ShowUserNamesOnSigninProto{
			ShowUserNames: &boolTrue,
		},
		DataRoamingEnabled: &enterprise_management_proto.DataRoamingEnabledProto{
			DataRoamingEnabled: &boolTrue,
		},
		AllowNewUsers: &enterprise_management_proto.AllowNewUsersProto{
			AllowNewUsers: &boolFalse,
		},
		UserWhitelist: &enterprise_management_proto.UserWhitelistProto{
			UserWhitelist: []string{user, "a@b.c"},
		},
	}
}

// UserPolicyDescriptor creates a PolicyDescriptor suitable for storing and
// retrieving user policy using session_manager's policy storage interface.
func UserPolicyDescriptor(accountID string) *lm.PolicyDescriptor {
	accountType := lm.PolicyAccountType_ACCOUNT_TYPE_USER
	domain := lm.PolicyDomain_POLICY_DOMAIN_CHROME
	return &lm.PolicyDescriptor{
		AccountType: &accountType,
		AccountId:   &accountID,
		Domain:      &domain,
	}
}
