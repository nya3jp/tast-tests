// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mlservice

import (
	"context"
	"fmt"
	"net/http"
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
			"qjw@chromium.org",     // Test author
			"honglinyu@google.com", // ml_service contact
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
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const localServerPort = 8081

	// Setup test HTTP server.
	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)
	server := &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local http server: ", err)
		}
	}()
	defer server.Shutdown(cleanupCtx)

	// Launch chrome.
	chromeOpts := []chrome.Option{
		// TODO(qjw): Change to EnableFeature after we add a flag (or feature) in Chrome browser.
		chrome.ExtraArgs("--enable-experimental-web-platform-features"),
	}

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}

	// Open the test page.
	conn, err := cr.NewConn(ctx, fmt.Sprintf("http://localhost:%v/web_handwriting_recognition_drawing_abc.html", localServerPort))
	if err != nil {
		s.Fatal("Failed to open test web page: ", err)
	}
	defer conn.Close()

	if err = conn.WaitForExpr(ctx, "window.resultPromise"); err != nil {
		s.Fatal("Failed to wait for test page result promise: ", err)
	}

	var result string
	if err = conn.Eval(ctx, "window.resultPromise", &result); err != nil {
		s.Fatal("Test result is failure: ", err)
	}
}
