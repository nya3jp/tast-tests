// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeFixture,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Demonstrates Chrome fixture",
		Contacts:     []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func ChromeFixture(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	const content = "Hooray, it worked!"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, content)
	}))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL)
	if err != nil {
		s.Fatal("Creating tab failed: ", err)
	}
	defer conn.Close()

	var actual string
	if err := conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
		s.Fatal("Getting page content failed: ", err)
	}
	if actual != content {
		s.Fatalf("Unexpected page content: got %q; want %q", actual, content)
	}
}
