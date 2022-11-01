// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uiauto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// screenshotsDir is the name of the directory where the screenshots
// will be saved.
const screenshotsDirName = "cuj-screenshots"

// ScreenshotRecorder is an interface used to take screenshots at
// specified intervals during a test.
type ScreenshotRecorder interface {
	// Start starts the screenshot recorder. The first screenshot is taken
	// after |interval| has passed. This function cannot be called on an
	// already started recorder without |Stop| being called first.
	Start(ctx context.Context) error

	// Stop stops the screenshot recorder. It takes one last screenshot to
	// capture the device at its end state, if the max number of
	// images for the recorder is greater than 0. This function can
	// only be called after |Start| has been called.
	Stop(ctx context.Context) error

	// TakeScreenshot takes a screenshot of the display and saves it
	// similarly to the screenshots taken at fixed intervals. This
	// screenshot, however, will have the "-custom" suffix, and can
	// be taken before and after |Start| and |Stop| has run.
	// TakeScreenshot counts towards the recorder's |maxImages|,
	// but is not limited by it. Thus, if TakeScreenshot is called
	// and we already hit the |maxImages|, the screenshot is still
	// taken, but no more shots would be taken at fixed intervals.
	TakeScreenshot(ctx context.Context)
}

// screenshotRecorderImpl is used to implement ScreenshotRecorder.
type screenshotRecorderImpl struct {
	// recording is whether or not the recorder has started recording.
	recording bool

	// takingIntervalShots is whether or not there is a goroutine
	// active that is taking screenshots at a given interval. The
	// screenshot goroutine will close when the max number of
	// screenshots have been taken. The overall recorder, however,
	// will remain active, so that it can appropriately cleanup
	// with |Stop|.
	takingIntervalShots bool

	// interval is the delay between screenshots.
	interval time.Duration

	// firstErr is the first error that occurred in the recorder.
	firstErr error

	// screenshotsDir is filepath to the directory where the
	// screenshots are saved.
	screenshotsDir string

	// startTime is the time the |Start| function is called.
	startTime time.Time

	// numImages is the number of images that have already been taken.
	numImages int

	// maxImages is the maximum number of images this recorder is
	// allowed to take. Setting this value is important to ensure
	// tests that are hanging for much longer than expected don't take
	// unexpected number of screenshots.
	maxImages int

	// stopc is the channel for the foreground task to stop the
	// background task.
	stopc chan struct{}

	// stopackc is the channel for the background task to tell the
	// foreground task that it is done.
	stopackc chan struct{}

	// customc is the channel for the foreground task to tell the
	// background task to take a screenshot.
	customc chan struct{}
}

// NewScreenshotRecorder creates a ScreenshotRecorder that can take
// screenshots of the display at the specified |interval|. If
// |interval| is 0, no interval screenshots will be taken. A screenshot
// will be taken in |Stop| if |maxImages| is greater than 0.
// After creating the recorder, you could start recording by calling
// |Start|, and stop recording by calling |Stop|.
//
// Examples:
// 1. NewScreenshotRecorder(ctx, time.Minute, 5) is a recorder that will
// take 1 screenshot every minute 4 times, and then take 1 screenshot at
// when |Stop| is called.
// 2. NewScreenshotRecorder(ctx, 0, 1) is a recorder that will take
// a screenshot when |Stop| is called.
// 3. NewScreenshotRecorder(ctx, 0, 0) is an empty recorder that will
// not take any screenshots.
//
// Examples #2 and #3 are useful when using TakeScreenshot, as that can
// give more flexibility to when screenshots are taken.
func NewScreenshotRecorder(ctx context.Context, interval time.Duration, maxImages int) (ScreenshotRecorder, error) {
	if maxImages < 0 {
		return nil, errors.Errorf("cannot create screenshot recorder with %d max images", maxImages)
	}

	if maxImages > 0 && interval <= 0 {
		return nil, errors.Errorf("cannot create screenshot recorder with a %v interval when max images is %d", interval, maxImages)
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return nil, errors.New("failed to get the out directory")
	}

	screenshotDir := filepath.Join(dir, screenshotsDirName)
	if err := os.Mkdir(screenshotDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create screenshot directory")
	}

	return &screenshotRecorderImpl{
		interval:       interval,
		screenshotsDir: screenshotDir,
		maxImages:      maxImages,
		customc:        make(chan struct{}),
		stopc:          make(chan struct{}),
		stopackc:       make(chan struct{}),
	}, nil
}

func (r *screenshotRecorderImpl) Start(ctx context.Context) error {
	if r.recording {
		return errors.New("screenshot recording has already started")
	}
	r.recording = true

	r.startTime = time.Now()

	go func() {
		r.takingIntervalShots = true

		// If an interval is not specified, we skip taking
		// screenshots at specified intervals.
		if r.interval == 0 {
			r.takingIntervalShots = false
		} else {
			testing.ContextLog(ctx, "screenshot_recorder: Taking a screenshot every ", r.interval)
		}

		for r.takingIntervalShots {
			// We subtract 1 from max images to reserve one
			// screenshot that is taken when |Stop| is called.
			if r.numImages >= r.maxImages-1 {
				testing.ContextLogf(ctx, "screenshot_recorder: The max number of interval screenshots have been taken (%d); will not take any more", r.maxImages-1)
				r.takingIntervalShots = false
				break
			}

			select {
			case <-time.After(r.interval):
				if err := r.captureDisplay(ctx, ""); err != nil {
					testing.ContextLog(ctx, "screenshot_recorder: Failed to take a screenshot: ", err)
				}
			case <-r.customc:
				r.customCaptureDisplay(ctx)
			case <-r.stopc:
				testing.ContextLog(ctx, "screenshot_recorder: Background signaled to stop")
				r.takingIntervalShots = false
			case <-ctx.Done():
				r.takingIntervalShots = false
			}
		}
		// Let the foreground task know we are done.
		close(r.stopackc)
	}()
	return nil
}

// Stop stops the screenshot recorder. It takes one last screenshot to
// capture the device at its end state. This function can only be
// called after |Start| has been called.
func (r *screenshotRecorderImpl) Stop(ctx context.Context) error {
	if !r.recording {
		return errors.New("start recording wasn't called before stop recording")
	}
	r.recording = false

	// Send stop message to screenshot routine.
	close(r.stopc)

	// Wait for confirmation that the screenshot recorder has stopped.
	var ctxIsFinished bool
	select {
	case <-time.After(30 * time.Second):
		return errors.New("timed out waiting for screenshot recorder to stop")
	case <-r.stopackc:
		testing.ContextLog(ctx, "screenshot_recorder: Screenshot recorder stopped successfully")
	case <-ctx.Done():
		ctxIsFinished = true
	}

	if r.firstErr != nil {
		return errors.Wrap(r.firstErr, "screenshot recording failed")
	}

	if !ctxIsFinished && r.maxImages > 0 {
		// Take a final screenshot. Append file name with "-end" to
		// explain why this screenshot was taken at a different
		// interval than the other screenshots.
		if err := r.captureDisplay(ctx, "-end"); err != nil {
			return errors.Wrap(err, "failed to take one last screenshot")
		}
	}

	return nil
}

func (r *screenshotRecorderImpl) TakeScreenshot(ctx context.Context) {
	// If we are still taking screenshots at specific intervals,
	// send a message to the screenshot goroutine to take a
	// screenshot. If we are not recording, take a screenshot
	// separately. By using a channel, we can prevent race
	// conditions, where two screenshots are taken at the same time.
	if r.takingIntervalShots {
		r.customc <- struct{}{}
	} else {
		r.customCaptureDisplay(ctx)
	}
}

// captureDisplay takes a screenshot of the active display and saves it
// to a file with the following format, where the screenshot sequence is
// what numbered screenshot this is in the test:
// cuj-<screenshot sequence>-<number of seconds since start><optional suffix>.jpg
// i.e.
// - 1st screenshot: cuj-1-10.jpg
// - 2nd screenshot: cuj-2-20-end.jpg
//
// Note: the screenshot numbering is based on only successful screenshots.
// i.e. with a screenshot that failed
// - 1st screenshot: cuj-1-10.jpg
// - 2nd screenshot: failed
// - 3rd screenshot: cuj-2-30.jpg
func (r *screenshotRecorderImpl) captureDisplay(ctx context.Context, fileSuffix string) error {
	testing.ContextLogf(ctx, "screenshot_recorder: Will take screenshot #%d", r.numImages+1)

	var secondsSinceStart int
	if !r.startTime.IsZero() {
		secondsSinceStart = int(time.Now().Sub(r.startTime).Seconds())
	}
	path := filepath.Join(r.screenshotsDir, fmt.Sprintf("cuj-%d-%d%s.jpg", r.numImages+1, secondsSinceStart, fileSuffix))

	cmd := testexec.CommandContext(ctx, "screenshot", path)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	r.numImages++

	if r.firstErr == nil {
		r.firstErr = err
	}
	return nil
}

// customCaptureDisplay is a wrapper around r.captureDisplay explicitly
// for custom screenshots. This function helps to eliminate code
// duplication between taking custom screenshots outside the screenshot
// goroutine and inside of it.
func (r *screenshotRecorderImpl) customCaptureDisplay(ctx context.Context) {
	if err := r.captureDisplay(ctx, "-custom"); err != nil {
		testing.ContextLog(ctx, "screenshot_recorder: Failed to take custom screenshot: ", err)
	}
}
