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
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// htmlFile is the file containing the HTML+JS code exercising getDisplayMedia().
	htmlFile = "getdisplaymedia.html"
)

// RunGetDisplayMedia drives the code verifying the getDisplayMedia functionality.
func RunGetDisplayMedia(ctx context.Context, s *testing.State, cr *chrome.Chrome, surfaceType string) error {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+htmlFile)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", htmlFile)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.Call(ctx, nil, "start", surfaceType); err != nil {
		return errors.Wrap(err, "failed to run getDisplayMedia()")
	}
	return nil
}

// DataFiles returns a list of files required to run the tests in this package.
func DataFiles() []string {
	return []string{
		htmlFile,
		"third_party/blackframe.js",
	}
}
