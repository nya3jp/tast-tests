// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	lm "chromiumos/system_api/login_manager_proto"
)

// MakeDevicePolicyDescriptor creates a PolicyDescriptor suitable for storing and
// retrieving device policy using Session Manager's policy storage interface.
func MakeDevicePolicyDescriptor() *lm.PolicyDescriptor {
	accountType := lm.PolicyAccountType_ACCOUNT_TYPE_DEVICE
	domain := lm.PolicyDomain_POLICY_DOMAIN_CHROME
	return &lm.PolicyDescriptor{
		AccountType: &accountType,
		Domain:      &domain,
	}
}

// MakeUserPolicyDescriptor creates a PolicyDescriptor suitable for storing and
// retrieving user policy using Session Manager's policy storage interface.
func MakeUserPolicyDescriptor(accountID string) *lm.PolicyDescriptor {
	accountType := lm.PolicyAccountType_ACCOUNT_TYPE_USER
	domain := lm.PolicyDomain_POLICY_DOMAIN_CHROME
	return &lm.PolicyDescriptor{
		AccountType: &accountType,
		AccountId:   &accountID,
		Domain:      &domain,
	}
}
