// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/common/policy"
	filesystemreadwrite "chromiumos/tast/local/bundles/cros/policy/file_system_read_write"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

const readBlockTestHTML = "file_system_read_for_urls_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemReadBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system reads are blocked depending on the value of this policy",
		Contacts: []string{
			"vivian.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"chromeos-commercial-remote-management@google.com",
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
		Data: []string{readBlockTestHTML},
	})
}

// FileSystemReadBlockedForUrls tests the FileSystemReadBlockedForUrls policy.
func FileSystemReadBlockedForUrls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to pharse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, readBlockTestHTML)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Blocked_matching test.
			URL:                url,
			WantFileSystemRead: false,
			Policy:             &policy.FileSystemReadBlockedForUrls{Val: []string{url}},
		},
		{
			// Blocked_non_matching test.
			URL:                url,
			WantFileSystemRead: true,
			Policy:             &policy.FileSystemReadBlockedForUrls{Val: []string{""}},
		},
		{
			// Unset test.
			URL:                url,
			WantFileSystemRead: true,
			Policy:             &policy.FileSystemReadBlockedForUrls{Stat: policy.StatusUnset},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
