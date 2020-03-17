// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/common"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: URLWhitelist,
		Desc: "Behavior of the URLWhitelist policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"vsavu@google.com",
			"enterprise-policy-support-rotation@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func URLWhitelist(ctx context.Context, s *testing.State) {
	testTable := []common.URLBlWhListTestTable{
		{
			Name:        "unset",
			BlockedURLs: []string{},
			AllowedURLs: []string{"http://chromium.org"},
			Policies: []policy.Policy{
				&policy.URLBlacklist{Stat: policy.StatusUnset},
				&policy.URLWhitelist{Stat: policy.StatusUnset},
			},
		},
		{
			Name:        "single",
			BlockedURLs: []string{"http://example.org"},
			AllowedURLs: []string{"http://chromium.org"},
			Policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"org"}},
				&policy.URLWhitelist{Val: []string{"chromium.org"}},
			},
		},
		{
			Name:        "identical",
			BlockedURLs: []string{"http://example.org"},
			AllowedURLs: []string{"http://chromium.org"},
			Policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"http://chromium.org", "http://example.org"}},
				&policy.URLWhitelist{Val: []string{"http://chromium.org"}},
			},
		},
		{
			Name:        "https",
			BlockedURLs: []string{"http://chromium.org"},
			AllowedURLs: []string{"https://chromium.org"},
			Policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"chromium.org"}},
				&policy.URLWhitelist{Val: []string{"https://chromium.org"}},
			},
		},
	}

	common.URLBlackWhitelist(ctx, s, testTable)
}
