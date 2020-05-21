// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webutil contains shared code for dealing with web content.
package webutil

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
)

// WaitForYoutubeVideo waits for a YouTube video to start on the given chrome.Conn.
func WaitForYoutubeVideo(ctx context.Context, conn *chrome.Conn, timeout time.Duration) error {
	// Wait for <video> tag to show up.
	return conn.WaitForExprFailOnErrWithTimeout(ctx, `
		(function() {
		  const v = document.querySelector('video');
		  if (!v) {
		    return false;
		  }
		  const bounds = v.getBoundingClientRect();
		  return bounds.x >= 0 && bounds.y >= 0 &&
		      bounds.width > 0 && bounds.height > 0;
		})()`, timeout)
}
