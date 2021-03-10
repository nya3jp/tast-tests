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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebHandwritingRecognition,
		Desc: "Checks Web Handwriting Recognition API works correctly with ml_service",
		Contacts: []string{
			"qjw@chromium.org",               // Test author
			"honglinyu@google.com",           // ml_service contact
			"handwriting-web-api@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome", "ondevice_handwriting"},
		Attr:         []string{"group:mainline", "informational"},
		Data: []string{
			"web_handwriting_recognition_drawing_abc.html",
			"web_handwriting_recognition_drawing_abc.json",
		},
	})
}

func WebHandwritingRecognition(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Launch chrome.
	// TODO(qjw): Change to EnableFeature after we add a flag (or feature) in Chrome browser.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-experimental-web-platform-features"))
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(ctx)

	// Open the test page.
	conn, err := cr.NewConn(ctx, server.URL+"/web_handwriting_recognition_drawing_abc.html")
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
