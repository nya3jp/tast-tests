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

const writeAskTestHTML = "file_system_write_for_urls_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemWriteAskForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system writes are allowed depending on the value of this policy",
		Contacts: []string{
			"vivian.tsai@cienet.com",
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
		Data: []string{writeAskTestHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.FileSystemWriteAskForUrls{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.DefaultFileSystemWriteGuardSetting{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// FileSystemWriteAskForUrls tests the FileSystemWriteAskForUrls policy.
func FileSystemWriteAskForUrls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, writeAskTestHTML)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Test of should ask for permission.
			Name:                 "ask",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{url}}},
		}, {
			// Test access granted for matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting block access.
			Name:                 "matching_defaultBlocked",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingBlock}},
		}, {
			// Test access granted for matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting allow access.
			Name:                 "matching_defaultAsk",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingAsk}},
		}, {
			// Test access granted for matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting unset.
			Name:                 "matching_defaultUnset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemWriteGuardSetting{Stat: policy.StatusUnset}},
		}, {
			// Test access denied for non-matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting block access.
			Name:                 "non_matching_defaultBlocked",
			URL:                  url,
			WantFileSystemAccess: false,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingBlock}},
		}, {
			// Test access granted for non-matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting allow access.
			Name:                 "non_matching_defaultAsk",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemWriteGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingAsk}},
		}, {
			// Test access granted for non-matching url in FileSystemWriteAskForUrls
			// with DefaultFileSystemWriteGuardSetting unset.
			Name:                 "non_matching_defaultUnset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemWriteGuardSetting{Stat: policy.StatusUnset}},
		}, {
			// Test of policy unset.
			Name:                 "unset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteAskForUrls{Stat: policy.StatusUnset}},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
