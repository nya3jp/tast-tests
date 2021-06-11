// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains functionality to return policy values for
// tests that excersice dlp restrictions.
package policy

import (
	"chromiumos/tast/common/policy"
)

// GetAllClipboardDlpPolicy returns clipboard policy with restricting all destinations.
func GetAllClipboardDlpPolicy() []policy.Policy {

	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in any destination",
				Description: "User should not be able to copy and paste confidential content in any destination",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"company.com",
						"chromium.org",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"*",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	return policyDLP
}

// GetStandardClipboardDlpPolicy returns standard clipboard dlp policy.
func GetStandardClipboardDlpPolicy() []policy.Policy {

	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in restricted destination",
				Description: "User should not be able to copy and paste confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"company.com",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"google.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	return policyDLP
}
