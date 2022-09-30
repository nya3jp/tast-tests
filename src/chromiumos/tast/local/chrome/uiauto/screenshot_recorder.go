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

// screenshotsDir is the name of the directory that the screenshots
// will be saved to.
const screenshotsDirName = "cuj-screenshots"

// ScreenshotRecorder is an interface used to take screenshots at
// specified intervals during a test.
type ScreenshotRecorder interface {
	// Start starts the screenshot recorder. The first screenshot is taken
	// after |interval| has passed. This function cannot be called on an
	// already started recorder without |Stop| being called first.
	Start(ctx context.Context) error

	// Stop stops the screenshot recorder. It takes one last screenshot to
	// capture the device at its end state. This function can only be
	// called after |Start| has been called.
	Stop(ctx context.Context) error
}

// screenshotRecorderImpl is used to implement ScreenshotRecorder.
type screenshotRecorderImpl struct {
	// recording is whether or not the recorder has started recording.
	recording bool

	// interval is the delay between screenshots.
	interval time.Duration

	// lastErr is the most recent error that occurred in the recorder.
	lastErr error

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
}

// NewScreenshotRecorder creates a ScreenshotRecorder that can take
// screenshots of the display at the specified |interval|. After
// creating the recorder, you could start recording by calling
// |Start|, and stop recording by calling |Stop|.
func NewScreenshotRecorder(ctx context.Context, interval time.Duration, maxImages int) (ScreenshotRecorder, error) {
	if maxImages <= 0 {
		return nil, errors.Errorf("cannot create screenshot recorder with %d max images", maxImages)
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return nil, errors.New("failed to get the out directory")
	}

	screenshotDir := filepath.Join(dir, screenshotsDirName)
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create screenshot directory")
	}

	return screenshotRecorderImpl{
		interval:       interval,
		screenshotsDir: screenshotDir,
		maxImages:      maxImages,
		stopc:          make(chan struct{}),
		stopackc:       make(chan struct{}),
	}, nil
}

func (r screenshotRecorderImpl) Start(ctx context.Context) error {
	if r.recording {
		return errors.New("screenshot recording has already started")
	}
	r.recording = true

	r.startTime = time.Now()

	testing.ContextLog(ctx, "screenshot_recorder: Taking a screenshot every ", r.interval)
	go func() {
		done := false
		for !done {
			select {
			case <-time.After(r.interval):
				r.lastErr = r.captureDisplay(ctx, "")
				if r.lastErr != nil {
					testing.ContextLog(ctx, "screenshot_recorder: Failed to take screenshot: ", r.lastErr)
					break
				}

				// We subtract 1 from max images to reserve one
				// screenshot that is taken when |Stop| is called.
				if r.numImages < r.maxImages-1 {
					break
				}

				testing.ContextLogf(ctx, "screenshot_recorder: The max number of screenshots have been taken (%d); will not take any more", r.maxImages)
				done = true
			case <-r.stopc:
				testing.ContextLog(ctx, "screenshot_recorder: Background signaled to stop")
				done = true
			case <-ctx.Done():
				done = true
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
func (r screenshotRecorderImpl) Stop(ctx context.Context) error {
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

	if r.lastErr != nil {
		return errors.Wrap(r.lastErr, "screenshot recording failed")
	}

	if !ctxIsFinished {
		// Take a final screenshot. Append file name with "-end" to
		// explain why this screenshot was taken at a different
		// interval than the other screenshots.
		if err := r.captureDisplay(ctx, "-end"); err != nil {
			return errors.Wrap(err, "failed to take one last screenshot")
		}
	}

	return nil
}

// captureDisplay takes a screenshot of the active display and saves it
// to a file with the following format, where the screenshot sequence is
// what numbered screenshot this is in the test:
// cuj-<screenshot sequence>-<number of seconds since start><optional suffix>.jpg
// i.e.
// - 1st screenshot: cuj-1-10.jpg
// - 2nd screenshot: cuj-2-20-end.jpg
func (r screenshotRecorderImpl) captureDisplay(ctx context.Context, fileSuffix string) error {
	testing.ContextLogf(ctx, "screenshot_recorder: Will take screenshot #%d", r.numImages+1)

	secondsSinceStart := int(time.Now().Sub(r.startTime).Seconds())
	path := filepath.Join(r.screenshotsDir, fmt.Sprintf("cuj-%d-%d%s.jpg", r.numImages+1, secondsSinceStart, fileSuffix))

	cmd := testexec.CommandContext(ctx, "screenshot", path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	r.numImages++
	return nil
}
