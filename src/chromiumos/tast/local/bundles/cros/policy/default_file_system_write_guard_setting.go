// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	filesystemreadwrite "chromiumos/tast/local/bundles/cros/policy/file_system_read_write"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

const writeGuardSettingTestHTML = "file_system_write_for_urls_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultFileSystemWriteGuardSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the DefaultFileSystemWriteGuardSetting policy",
		Contacts: []string{
			"bob.yang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{writeGuardSettingTestHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultFileSystemWriteGuardSetting{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// DefaultFileSystemWriteGuardSetting tests the DefaultFileSystemWriteGuardSetting policy.
func DefaultFileSystemWriteGuardSetting(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, writeGuardSettingTestHTML)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Test of access blocked.
			Name:                 "blocked",
			URL:                  url,
			WantFileSystemAccess: false,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingBlock}},
		}, {
			// Test of access granted.
			Name:                 "ask",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingAsk}},
		}, {
			// Test of access granted when status unset.
			Name:                 "unset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.DefaultFileSystemWriteGuardSetting{Stat: policy.StatusUnset}},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
