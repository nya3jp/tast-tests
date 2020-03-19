// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package getdisplaymedia provides common code for WebRTC's getDisplayMedia
// tests; this API is used for screen, window and tab capture, see
// https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getDisplayMedia
// and https://w3c.github.io/mediacapture-screen-share/.
package getdisplaymedia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// HTMLFile is the file containing the HTML+JS code exercising getDisplayMedia().
	HTMLFile = "getdisplaymedia.html"
)

// RunGetDisplayMedia launches the code for verifying the getDisplayMedia functionality.
func RunGetDisplayMedia(ctx context.Context, s *testing.State, cr *chrome.Chrome, surfaceType string) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

    	conn, err := cr.NewConn(ctx, server.URL+"/"+"getdisplaymedia.html")
	if err != nil {
		s.Fatal("Failed to open about:blank:  ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("start(%q)", surfaceType), nil); err != nil {
		s.Fatal("failed to run getDisplayMedia(): ", err)
	}
}
