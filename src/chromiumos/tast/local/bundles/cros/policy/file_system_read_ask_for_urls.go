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

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemReadAskForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system reads are allowed depending on the value of this policy",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
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
		Data: []string{"file_system_read_for_urls_index.html"},
	})
}

// FileSystemReadAskForUrls tests the FileSystemReadAskForUrls policy.
func FileSystemReadAskForUrls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	const indexFileName = "file_system_read_for_urls_index.html"
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to pharse url: ", err)
	}
	baseURL.Path = filepath.Join(baseURL.Path, indexFileName)
	url := baseURL.String()

	for _, param := range []filesystemreadwrite.TestCase{
		{
			// Set test.
			URL:                url,
			WantFileSystemRead: true,
			Policy:             &policy.FileSystemReadAskForUrls{Val: []string{url}},
		},
		{
			// Unset test.
			URL:                url,
			WantFileSystemRead: true,
			Policy:             &policy.FileSystemReadAskForUrls{Stat: policy.StatusUnset},
		},
	} {
		filesystemreadwrite.RunTestCases(ctx, s, param)
	}
}
