// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains functionality to return policy values for
// tests that excersice dlp restrictions.
package policy

import (
	"chromiumos/tast/common/policy"
)

// RestrictiveDLPPolicyForClipboard returns a clipboard policy blocking clipboard from source to all destination urls.
func RestrictiveDLPPolicyForClipboard(source string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in any destination",
				Description: "User should not be able to copy and paste confidential content in any destination",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						source,
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

// ClipboardWarnPolicy returns a clipboard dlp policy warning when clipboard content is copied and pasted from source to destination.
func ClipboardWarnPolicy(source, destination string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn about copy and paste of confidential content in restricted destination",
				Description: "User should be warned when coping and pasting confidential content in restricted destination",
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
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}

// ClipboardBlockPolicy returns a clipboard dlp policy warning when clipboard content is copied and pasted from source to destination.
func ClipboardBlockPolicy(source, destination string) []policy.Policy {
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

// PrintingBlockPolicy returns policy that blocks printing based on given url
func PrintingBlockPolicy(url string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable printing of confidential content",
				Description: "User should not be able to print confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						url,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "PRINTING",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// PrintingWarnPolicy returns policy that warns in case of printing based on given url
func PrintingWarnPolicy(url string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn before printing confidential content",
				Description: "User should be warned before printing confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						url,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "PRINTING",
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}
