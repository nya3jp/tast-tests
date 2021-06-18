// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"chromiumos/tast/common/policy"
)

// NewDLPPolicy returns list of screenshot paths in Download folder.
func NewDLPPolicy(policyName, policyDescription string, restrictions, sourcesURL, destinationsURL []string) []policy.Policy {
	var policyRestrictions []*policy.DataLeakPreventionRulesListRestrictions
	for _, value := range restrictions {
		tmp := &policy.DataLeakPreventionRulesListRestrictions{
			Class: value,
			Level: "BLOCK",
		}
		policyRestrictions = append(policyRestrictions, tmp)
	}
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        policyName,
				Description: policyDescription,
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: sourcesURL,
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: destinationsURL,
				},
				Restrictions: policyRestrictions,
			},
		},
	},
	}

	return policyDLP
}
