// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains functionality to return policy values for
// tests that excersice dlp restrictions.
package policy

import (
	"chromiumos/tast/common/policy"
)

// RestrictiveDLPPolicyForClipboard returns clipboard policy restricting all destinations.
func RestrictiveDLPPolicyForClipboard() []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in any destination",
				Description: "User should not be able to copy and paste confidential content in any destination",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"example.com",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Urls: []string{
						"*",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// PopulateDLPPolicyForClipboard returns a clipboard dlp policy blocking clipboard from source to destination.
func PopulateDLPPolicyForClipboard(source, destination string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in restricted destination",
				Description: "User should not be able to copy and paste confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						source,
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Urls: []string{
						destination,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// StandardDLPPolicyForClipboard returns the standard clipboard dlp policy.
func StandardDLPPolicyForClipboard() []policy.Policy {
	return PopulateDLPPolicyForClipboard("example.com", "google.com")
}
