// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package getdisplaymedia provides common code for webrtc.* GetDisplayMedia tests.
// See https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getDisplayMedia
// and https://w3c.github.io/mediacapture-screen-share/.
package getdisplaymedia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

// RunGetDisplayMedia launches...
func RunGetDisplayMedia(ctx context.Context, s *testing.State, cr *chrome.Chrome, surfaceType string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

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
