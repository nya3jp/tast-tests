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

const writeBlockTestHTML = "file_system_write_for_urls_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemWriteBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system writes are blocked depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
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
		Data: []string{writeBlockTestHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.FileSystemWriteBlockedForUrls{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// FileSystemWriteBlockedForUrls tests the FileSystemWriteBlockedForUrls policy.
func FileSystemWriteBlockedForUrls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, writeBlockTestHTML)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Test of writing permission, which will be blocked at matching url.
			Name:                 "blocked_matching",
			URL:                  url,
			WantFileSystemAccess: false,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteBlockedForUrls{Val: []string{url}}},
		}, {
			// Test of writing permission, which will be blocked at non-matching url.
			Name:                 "blocked_non_matching",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteBlockedForUrls{Val: []string{""}}},
		}, {
			// Test of policy unset.
			Name:                 "unset",
			URL:                  url,
			WantFileSystemAccess: true,
			Method:               filesystemreadwrite.Write,
			Policies: []policy.Policy{
				&policy.FileSystemWriteBlockedForUrls{Stat: policy.StatusUnset}},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
