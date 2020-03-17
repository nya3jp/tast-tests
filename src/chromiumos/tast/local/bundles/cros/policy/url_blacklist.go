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
		Func: URLBlacklist,
		Desc: "Behavior of the URLBlacklist policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func URLBlacklist(ctx context.Context, s *testing.State) {
	testTable := []common.URLBlWhListTestTable{
		{
			Name:        "unset",
			BlockedURLs: []string{},
			AllowedURLs: []string{"http://google.com", "http://chromium.org"},
			Policies:    []policy.Policy{&policy.URLBlacklist{Stat: policy.StatusUnset}},
		},
		{
			Name:        "single",
			BlockedURLs: []string{"http://example.org/blocked.html"},
			AllowedURLs: []string{"http://google.com", "http://chromium.org"},
			Policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"http://example.org/blocked.html"}}},
		},
		{
			Name:        "multi",
			BlockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
			AllowedURLs: []string{"http://google.com", "http://chromium.org"},
			Policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}}},
		},
		{
			Name:        "wildcard",
			BlockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
			AllowedURLs: []string{"http://google.com", "http://chromium.org"},
			Policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"example.com"}}},
		},
	}

	common.URLBlackWhitelist(ctx, s, testTable)
}
