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
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

// GetFramedrops is a function that get framedrops.
type GetFramedrops func(context.Context, *chrome.Conn) (framedrops int, err error)

// isPlayingWithDebugMode verifies media is playing by checking mediaElement.currentTime after the given duration to see
// if currentTime is moving forward. The actual video playing lengtg might be smaller than the time elapses fore various
// reasons, so a bufferRatio is given to tolerate the video buffering time.
// With isDebugMode on will take screenshot for more debug information.
func isPlayingWithDebugMode(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string, isDebugMode bool) error {
	if isDebugMode {
		testing.ContextLog(ctx, "Starting to take first screenshot")
		debug.TakeScreenshot(ctx)
	}
	previousTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	currentTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	bufferRatio := 0.33
	bufferedTimeout := timeout.Seconds() * bufferRatio
	if isDebugMode {
		testing.ContextLog(ctx, "Starting to take second screenshot")
		debug.TakeScreenshot(ctx)
	}
	if currentTime == previousTime {
		if isDebugMode {
			testing.ContextLog(ctx, "Starting to take third screenshot due to media element did not play at all")
			debug.TakeScreenshot(ctx)
			testing.ContextLog(ctx, "Returns a new mtbf error")
		}
		// Not played at all.
		return mtbferrors.New(mtbferrors.VideoNoPlay, nil, currentTime, previousTime)
	} else if (math.Round(currentTime) - math.Round(previousTime)) < bufferedTimeout {
		// Played, but progress is too small.
		return mtbferrors.New(mtbferrors.VideoPlaying, nil, currentTime, previousTime, bufferedTimeout, timeout.Seconds())
	}

	return nil
}

// IsPlaying verifies media is playing by checking mediaElement.currentTime after the given duration to see
// if currentTime is moving forward. The actual video playing lengtg might be smaller than the time elapses fore various
// reasons, so a bufferRatio is given to tolerate the video buffering time.
func IsPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string) error {
	return isPlayingWithDebugMode(ctx, conn, timeout, selector, true)
}

// IsPlayingWithoutSnapshot verifies media is playing by checking mediaElement.currentTime after the given duration to see
// if currentTime is moving forward. The actual video playing lengtg might be smaller than the time elapses fore various
// reasons, so a bufferRatio is given to tolerate the video buffering time.
func IsPlayingWithoutSnapshot(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string) error {
	return isPlayingWithDebugMode(ctx, conn, timeout, selector, false)
}

// IsPausing verifies media is pausing by checking mediaElement.currentTime over the given timeout duration.
func IsPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration, selector string) error {
	startTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}

	var endTime float64
	retry := 0
	for retry < 2 {
		retry++

		endTime, err = dom.GetElementCurrentTime(ctx, conn, selector)
		if err != nil {
			return mtbferrors.New(mtbferrors.VideoGetTime, err)
		}
		if endTime == startTime {
			return nil
		}
	}

	return mtbferrors.New(mtbferrors.VideoVeriPause, nil, startTime, endTime)
}

// PauseAndResumeWithDebugMode first puase the media playing, and then resumes it.
// With isDebugMode on will take screenshot for more debug information.
func PauseAndResumeWithDebugMode(ctx context.Context, conn *chrome.Conn, selector string, isDebugMode bool) error {
	if err := dom.PauseElement(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.VideoPauseElement, err)
	}
	if err := IsPausing(ctx, conn, 3*time.Second, selector); err != nil {
		return err
	}
	if err := dom.PlayElement(ctx, conn, selector); err != nil {
		return mtbferrors.New(mtbferrors.VideoPlayElement, err)
	}
	if err := isPlayingWithDebugMode(ctx, conn, 3*time.Second, selector, isDebugMode); err != nil {
		return err
	}

	return nil
}

// PauseAndResume first puase the media playing, and then resumes it.
func PauseAndResume(ctx context.Context, conn *chrome.Conn, selector string) error {
	return PauseAndResumeWithDebugMode(ctx, conn, selector, true)
}

// FastJump does a fast jump and verifies the media element current time is correct.
func FastJump(ctx context.Context, conn *chrome.Conn, selector string, jumpTime float64) error {
	startTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	if err = dom.FastJumpElement(ctx, conn, selector, jumpTime); err != nil {
		return mtbferrors.New(mtbferrors.VideoFastJump, err)
	}
	endTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}

	tolerance := 0.5
	actualJump := math.Abs(endTime - startTime)
	expectedJump := math.Abs(jumpTime)
	if actualJump <= expectedJump-tolerance || actualJump >= expectedJump+tolerance {
		return mtbferrors.New(mtbferrors.VideoJumpTo, nil, startTime, endTime)
	}
	return nil
}

// FastForward does a fast forward and verifies the media element current time is correct.
func FastForward(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJump(ctx, conn, selector, dom.FastForwardTime)
}

// FastRewind does a fast rewind and verifies the media element current time is correct.
func FastRewind(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJump(ctx, conn, selector, dom.FastRewindTime)
}

// RandomSeek randomly moves mediaElement.currentTime to see if onseeked event works properly.
func RandomSeek(ctx context.Context, conn *chrome.Conn, numSeeks int, selector string) error {
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
				video.currentTime = Math.random() * 0.8 * video.duration;
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
			return mtbferrors.New(mtbferrors.VideoSeeks, err, finishedSeeks, numSeeks)
		}
		if finishedSeeks == numSeeks {
			break
		}
		prevFinishedSeeks = finishedSeeks
	}

	return nil
}

// CheckFramedrops checks frame drops every second for a given duration.
// Error will be returned based on number of frames dropped.
func CheckFramedrops(ctx context.Context, conn *chrome.Conn, timeout time.Duration, fps int, selector string, getFramedrop GetFramedrops) error {
	currentTime, err := dom.GetElementCurrentTime(ctx, conn, selector)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	var (
		interval  = 1 * time.Second
		totalTime time.Duration
	)

	totalFramedrops := 0
	for totalTime <= timeout {
		totalTime += interval
		if err = testing.Sleep(ctx, interval); err != nil {
			return mtbferrors.New(mtbferrors.ChromeSleep, err)
		}

		framedrops, err := getFramedrop(ctx, conn)
		if err != nil {
			return mtbferrors.New(mtbferrors.VideoGetFrmDrop, err)
		}
		totalFramedrops = framedrops
	}

	totalFrames := fps * int(timeout.Seconds())
	framedropsPercentage := float64(totalFramedrops) / float64(totalFrames) * 100
	testing.ContextLogf(ctx, "Total frames(%d) frame drops(%d), framedrops percentage(%0.2f)", totalFrames, totalFramedrops, framedropsPercentage)

	const (
		minorFrameDropThreshold  = 5
		majorFrameDropThreshold  = 10
		severeFrameDropThreshold = 20
	)
	if framedropsPercentage >= severeFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoSevereFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if framedropsPercentage >= majorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMajorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if framedropsPercentage >= minorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMinorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	}

	return nil
}
