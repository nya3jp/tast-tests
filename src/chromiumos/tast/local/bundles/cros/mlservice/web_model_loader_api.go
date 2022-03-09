// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func:         WebModelLoaderAPI,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks Web Model Loader API works correctly with ml_service",
		Contacts: []string{
			"honglinyu@google.com",               // test author
			"web-ml-model-loader-api@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{},
	})
}

func WebModelLoaderAPI(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Launch chrome.
	cr, err := chrome.New(
		ctx,
		chrome.ExtraArgs("--enable-experimental-web-platform-features"),
	)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Open the test page.
	conn, err := cr.NewConn(ctx, "https://false-shy-event.glitch.me/")
	if err != nil {
		s.Fatal("Failed to open test web page: ", err)
	}
	defer conn.Close()

	// Wait for JavaScript test to start. The test exposes `resultPromise`, which either
	// resolves to true (indicating PASS), or rejects with an error message (indicating
	// FAILURE).
	if err = conn.WaitForExpr(ctx, "IsInferenceDone()"); err != nil {
		s.Fatal("Failed to wait for test page result promise: ", err)
	}
}
