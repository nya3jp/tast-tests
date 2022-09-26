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

const readAskTestHTML = "file_system_read_for_urls_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemReadAskForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system reads are allowed depending on the value of this policy",
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
		Data: []string{readAskTestHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultFileSystemReadGuardSetting{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.FileSystemReadAskForUrls{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// FileSystemReadAskForUrls tests the FileSystemReadAskForUrls policy.
func FileSystemReadAskForUrls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, readAskTestHTML)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Set allowed matching test.
			Name:                 "ask",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{url}}},
		}, {
			// Test access granted for matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting block access.
			Name:                 "matching_defaultBlocked",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemReadGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingBlock}},
		}, {
			// Test access granted for matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting allow access.
			Name:                 "matching_defaultAsk",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemReadGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingAsk}},
		}, {
			// Test access granted for matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting unset.
			Name:                 "matching_defaultUnset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{url}},
				&policy.DefaultFileSystemReadGuardSetting{Stat: policy.StatusUnset}},
		}, {
			// Test access denied for non-matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting block access.
			Name:                 "non_matching_defaultBlocked",
			URL:                  url,
			WantFileSystemAccess: false,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemReadGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingBlock}},
		}, {
			// Test access granted for non-matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting allow access.
			Name:                 "non_matching_defaultAsk",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemReadGuardSetting{Val: filesystemreadwrite.DefaultGuardSettingAsk}},
		}, {
			// Test access granted for non-matching url in FileSystemReadAskForUrls
			// with DefaultFileSystemReadGuardSetting unset.
			Name:                 "non_matching_defaultUnset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Val: []string{""}},
				&policy.DefaultFileSystemReadGuardSetting{Stat: policy.StatusUnset}},
		}, {
			// Unset test.
			Name:                 "unset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Read,
			Policies: []policy.Policy{
				&policy.FileSystemReadAskForUrls{Stat: policy.StatusUnset}},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
