// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package media

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil/dom"
	"chromiumos/tast/testing"
)

// VerifyPlayingElement by checking mediaElement.currentTime every 1 second for a period of time to see if currentTime is moving forward.
func VerifyPlayingElement(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string) (err error) {
	var currentTime, previousTime float64
	if previousTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return
	}
	if currentTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}

	var (
		bufferRatio     = 0.33
		bufferedTimeout = timeout.Seconds() * bufferRatio
	)
	if currentTime == previousTime {
		return mtbferrors.New(mtbferrors.Err3330, nil, currentTime, previousTime)
	} else if (math.Round(currentTime) - math.Round(previousTime)) < bufferedTimeout {
		return mtbferrors.New(mtbferrors.Err3331, nil, currentTime, previousTime, bufferedTimeout, timeout.Seconds())
	}

	return nil
}

// VerifyPausingElement by checking mediaElement.currentTime is moving or not.
func VerifyPausingElement(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string) (err error) {
	var startTime, endTime float64
	if startTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}
	testing.Sleep(ctx, timeout)
	if endTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}

	if endTime-startTime != 0 {
		// Second chance.
		if endTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
			return mtbferrors.New(mtbferrors.Err3327, err)
		}
		if endTime-startTime != 0 {
			return mtbferrors.New(mtbferrors.Err3329, nil, startTime, endTime)
		}
	}

	return nil
}

// VerifyPauseAndResumeElement combines VerifyPlaying and VerifyPausing
func VerifyPauseAndResumeElement(ctx context.Context, conn *chrome.Conn, selector string) (err error) {
	if err = dom.PauseElement(ctx, conn, selector); err != nil {
		return
	}
	if err = VerifyPausingElement(ctx, conn, 3*time.Second, selector); err != nil {
		return
	}
	if err = dom.PlayElement(ctx, conn, selector); err != nil {
		return
	}
	if err = VerifyPlayingElement(ctx, conn, 3*time.Second, selector); err != nil {
		return
	}

	return nil
}

//VerifyFastJumpElement does a fast jump and verify it's current time is correct.
func VerifyFastJumpElement(ctx context.Context, conn *chrome.Conn, selector string, jumpTime float64) (err error) {
	var startTime, endTime float64
	if startTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}
	if err = dom.FastJumpElement(ctx, conn, selector, jumpTime); err != nil {
		return mtbferrors.New(mtbferrors.Err3332, err)
	}
	if endTime, err = dom.GetElementCurrentTime(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.Err3326, err)
	}
	if math.Round(math.Abs(endTime-startTime)) != math.Abs(jumpTime) {
		return mtbferrors.New(mtbferrors.Err3328, nil, startTime, endTime)
	}
	return nil
}

// VerifyFastForwardElement does a fast forward and verify it's current time is correct.
func VerifyFastForwardElement(ctx context.Context, conn *chrome.Conn, selector string) (err error) {
	return VerifyFastJumpElement(ctx, conn, selector, dom.FastForwardTime)
}

// VerifyFastRewindElement does a fast rewind and v erify it's current time is correct.
func VerifyFastRewindElement(ctx context.Context, conn *chrome.Conn, selector string) (err error) {
	return VerifyFastJumpElement(ctx, conn, selector, dom.FastRewindTime)
}

// VerifyRandomSeekingElement by randomly moving mediaElement.currentTime to see if onseeked event works properly.
func VerifyRandomSeekingElement(ctx context.Context, conn *chrome.Conn, numSeeks int, selector string) error {
	script := fmt.Sprintf(`(function randomSeek() {
			const video = %s
			let number_finished_seeks = 0;
			return new Promise((resolve, reject) => {
				video.onseeked = (event) => {
					console.log(number_finished_seeks);
					resolve(number_finished_seeks++);
				};
				video.onerror = (event) => {
					reject(new Error('Video error ' + event.error));
				};
				video.currentTime = Math.random() * video.duration;
			});
		})()`, dom.Query(selector))

	prevFinishedSeeks := 0
	for i := 0; i < numSeeks; i++ {
		finishedSeeks := 0
		if err := conn.EvalPromise(ctx, script, &finishedSeeks); err != nil {
			// If the test times out, EvalPromise() might be interrupted and return
			// zero finishedSeeks, in that case used the last known good amount.
			if finishedSeeks == 0 {
				finishedSeeks = prevFinishedSeeks
			}
			return mtbferrors.New(mtbferrors.Err3325, err, finishedSeeks, numSeeks)
		}
		if finishedSeeks == numSeeks {
			break
		}
		prevFinishedSeeks = finishedSeeks
	}

	return nil
}
