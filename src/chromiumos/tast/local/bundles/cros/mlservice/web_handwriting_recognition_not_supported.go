// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mlservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebHandwritingRecognitionNotSupported,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Web Handwriting Recognition API works, even if ml_service doesn't support ondevice_handwriting",
		Contacts: []string{
			"qjw@chromium.org",               // Test author
			"honglinyu@google.com",           // ml_service contact
			"handwriting-web-api@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome", "no_ondevice_handwriting"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Data: []string{
			"web_handwriting_recognition_not_supported.html",
		},
	})
}

func WebHandwritingRecognitionNotSupported(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Open browser.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open the test page.
	conn, err := br.NewConn(ctx, server.URL+"/web_handwriting_recognition_not_supported.html")
	if err != nil {
		s.Fatal("Failed to open test web page: ", err)
	}
	defer conn.Close()

	// Wait for JavaScript test to start. The test exposes `resultPromise`, which either
	// resolves to true (indicating PASS), or rejects with an error message (indicating
	// FAILURE).
	if err = conn.WaitForExpr(ctx, "'resultPromise' in window"); err != nil {
		s.Fatal("Failed to wait for test page result promise: ", err)
	}

	if err = conn.Eval(ctx, "window.resultPromise", nil); err != nil {
		s.Fatal("Failed to complete JS test: ", err)
	}
}
