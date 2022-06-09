// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/local/input"
)

// ScrollDownFor two-finger swipes on the trackpad to scroll down repeatedly until the
// scrollDuration has passed, with a scrollDelay between each swipe.
func ScrollDownFor(ctx context.Context, tpw *input.TrackpadEventWriter, tw *input.TouchEventWriter, scrollDelay, scrollDuration time.Duration) error {
	fingerSpacing := tpw.Width() / 4
	fingerNum := 2

	var startX, startY, endX, endY input.TouchCoord
	startX, startY, endX, endY = tpw.Width()/2, 1, tpw.Width()/2, tpw.Height()-1

	for endTime := time.Now().Add(scrollDuration); time.Now().Before(endTime); {
		// Double swipe from the middle button to the middle top of the touchpad.
		if err := tw.Swipe(ctx, startX, startY, endX, endY, fingerSpacing,
			fingerNum, scrollDelay); err != nil {
			return err
		}
	}
	return nil
}
