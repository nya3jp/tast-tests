// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package companionlib provides helper functions for companionlib tast tests.
package companionlib

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// SetWhiteWallpaper will sets the wallpaper to be a white image.
func SetWhiteWallpaper(ctx context.Context, tconn *chrome.TestConn, s *testing.State) error {
	const wallpaper = "white_wallpaper.jpg"

	// Using HTTP server to provide image for wallpaper setting, because this chrome API don't support local file and gs file.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	return SetWallpaper(ctx, tconn, server.URL+"/"+wallpaper)
}

// setWallpaper setting given URL as ChromeOS wallpaper.
func setWallpaper(ctx context.Context, tconn *chrome.TestConn, wallpaperURL string) error {
	return tconn.Call(ctx, nil, `(url) => tast.promisify(chrome.wallpaper.setWallpaper)({
		  url: url,
		  layout: 'STRETCH',
		  filename: 'test_wallpaper'
		})`, wallpaperURL)
}
