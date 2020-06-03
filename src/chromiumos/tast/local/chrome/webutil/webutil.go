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

// WaitForQuiescence waits for the given chrome.Conn gets quiescence.
func WaitForQuiescence(ctx context.Context, conn *chrome.Conn, timeout time.Duration) error {
	// Each resourceTimings element contains the load start time and load end time
	// for a resource.  If a load has not completed yet, the end time is set to
	// the current time.  Then we can tell that a load has completed by detecting
	// that the end time diverges from the current time.
	//
	// resourceTimings is sorted by event start time, so we need to look through
	// the entire array to find the latest activity.
	return conn.WaitForExprFailOnErrWithTimeout(ctx, `
		(function() {
			if (document.readyState !== 'complete') {
				return false;
			}

			const QUIESCENCE_TIMEOUT_MS = 2000;
			let lastEventTime = performance.timing.loadEventEnd -
					performance.timing.navigationStart;
			const resourceTimings = performance.getEntriesByType('resource');
			lastEventTime = resourceTimings.reduce(
					(current, timing) => Math.max(current, timing.responseEnd),
					lastEventTime);
			return performance.now() >= lastEventTime + QUIESCENCE_TIMEOUT_MS;
		})()`, timeout)
}
