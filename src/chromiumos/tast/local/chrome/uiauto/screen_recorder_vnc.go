// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uiauto

import (
	"context"
	"image"
	"net"
	"time"

	vnc "github.com/matts1/vnc2video"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/kmsvnc"
	"chromiumos/tast/testing"
)

// The worst case ratio between the amount of time required to encode a video and the video duration.
// For example, 0.3 would mean it takes at most 0.3 seconds to encode 1 second of video.
const encodingToVideoRatio = 0.3
const encodingToTestDurationRatio = encodingToVideoRatio / (encodingToVideoRatio + 1)

type videoConfig struct {
	fileName        string
	framerate       int
	recordOnSuccess bool
}

var defaultConfig = videoConfig{
	fileName:        "recording.webm",
	framerate:       10,
	recordOnSuccess: false,
}

// RecordingFileName sets the filename of the video recording (default is recording.webm).
func RecordingFileName(fileName string) func(*videoConfig) {
	return func(c *videoConfig) {
		c.fileName = fileName
	}
}

// RecordingFramerate sets the framerate of the video recording.
func RecordingFramerate(framerate int) func(*videoConfig) {
	return func(c *videoConfig) {
		c.framerate = framerate
	}
}

// RecordOnSuccess ensures that we record video even if the test succeeds.
func RecordOnSuccess() func(*videoConfig) {
	return func(c *videoConfig) {
		c.recordOnSuccess = true
	}
}

// ReserveForVNCRecordingCleanup shortens the context for vnc video encoding.
// It calculates the amount of time required to clean up, and calls
// ctxutil.Shorten by that amount.
func ReserveForVNCRecordingCleanup(ctx context.Context) (context.Context, context.CancelFunc) {
	dl, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	timeLeft := time.Until(dl)
	encodingTimeRequired := time.Duration(float64(timeLeft.Nanoseconds()) * encodingToTestDurationRatio)
	// Allow 4 seconds to kill kmsvnc.
	return ctxutil.Shorten(ctx, encodingTimeRequired+4*time.Second)
}

// RecordVNCVideo starts recording video from a VNC stream.
// It returns a function that will stop recording the video.
// Example usage:
// stopRecording := RecordVNCVideo(ctx, s, RecordingFramerate(5))
// defer stopRecording()
func RecordVNCVideo(ctx context.Context, s testingState, mods ...func(*videoConfig)) (stopRecording func()) {
	stopRecording, err := RecordVNCVideoCritical(ctx, s, mods...)
	if err != nil {
		testing.ContextLog(ctx, "Error while starting screen recording: ", err)
		return func() {}
	}
	return stopRecording
}

// RecordVNCVideoCritical starts recording video from a VNC stream.
// If the recording was unable to start, an error will be returned.
// Otherwise, a function to stop and save the recording will be returned.
// If the stop recording function fails, it will be logged, but as it is
// non-critical to the test itself, the test will still pass.
// Example usage:
// stopRecording, err := RecordVNCVideoCritical(ctx, s, RecordingFramerate(5))
// if err != nil {
// 	handle err
// }
// defer stopRecording()
func RecordVNCVideoCritical(ctx context.Context, s testingState, mods ...func(*videoConfig)) (stopRecording func(), err error) {
	cfg := defaultConfig
	for _, mod := range mods {
		mod(&cfg)
	}

	kms, err := kmsvnc.NewKmsvnc(ctx, false)
	if err != nil {
		return nil, err
	}

	cleanups := []func(){func() {
		if err := kms.Stop(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop kmsvnc: ", err)
		}
	}}
	// This is basically a fake defer, since we can't use defer because
	// if all goes well, this gets called when we stop recording, not
	// when this function ends.
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	serverMessageCh := make(chan vnc.ServerMessage)
	errorCh := make(chan error)
	quitCh := make(chan struct{})
	enc := vnc.NewTightEncoder()

	ccfg := &vnc.ClientConfig{
		SecurityHandlers: []vnc.SecurityHandler{
			&vnc.ClientAuthNone{},
		},
		DrawCursor:      true,
		PixelFormat:     vnc.PixelFormat32bit,
		ServerMessageCh: serverMessageCh,
		Messages:        vnc.DefaultServerMessages,
		Encodings: []vnc.Encoding{
			&enc,
			// This encoding isn't actually used, but if you tell it you're handling
			// the cursor it won't try and put the cursor into the video stream itself.
			&vnc.CursorPseudoEncoding{},
		},
		ErrorCh: errorCh,
		QuitCh:  quitCh,
	}

	nc, err := net.DialTimeout("tcp", "localhost:5900", 5*time.Second)
	if err != nil {
		cleanup()
		return nil, err
	}
	// Don't add nc.Close to cleanup, since it gets owned by the vnc.Connect.

	cc, err := vnc.Connect(ctx, nc, ccfg)
	if err != nil {
		if errCleanup := nc.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close localhost:5900: ", errCleanup)
		}
		cleanup()
		return nil, errors.Wrap(err, "error connecting to vnc host")
	}
	cleanups = append(cleanups, func() {
		if err := cc.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close VNC connection: ", err)
		}
	})

	if err := cc.SetEncodings([]vnc.EncodingType{
		vnc.EncCursorPseudo,
		vnc.EncTight,
	}); err != nil {
		cleanup()
		return nil, err
	}

	startTime := time.Time{}
	recordingStart := make(chan struct{})

	var errs []error

	go func() {
		for {
			select {
			case <-quitCh:
				return
			case err := <-errorCh:
				err = errors.Wrap(err, "received error during screen recording - stopping screen recording and continuing test")
				testing.ContextLog(ctx, err.Error())
				errs = append(errs, err)
				// By returning, we simply ensure that we don't ask for any more frames.
				// This way, at the end we will still encode video for the frames we do have.
				return
			case msg := <-serverMessageCh:
				if msg.Type() == vnc.FramebufferUpdateMsgType {
					if startTime.Equal(time.Time{}) {
						startTime = time.Now()
						recordingStart <- struct{}{}
					}
					// The VNC RFB protocol requires that we finish processing a frame before we request a new one.
					reqMsg := vnc.FramebufferUpdateRequest{Inc: 1, X: 0, Y: 0, Width: cc.Width(), Height: cc.Height()}
					if err := reqMsg.Write(cc); err != nil {
						err = errors.Wrap(err, "failed to request an additional frame")
						testing.ContextLog(ctx, err.Error())
						errs = append(errs, err)
						return
					}
				}
			}
		}
	}()

	select {
	case err := <-errorCh:
		cleanup()
		return nil, err
	case <-recordingStart:
		testing.ContextLog(ctx, "Started screen recording")
	}

	return func() {
		if !cfg.recordOnSuccess && !s.HasError() {
			cleanup()
			return
		}

		// VNC stream is slightly behind. Allow a second to capture everything that happened since.
		// Potential room for improvement: start encoding video before this (it's only 1 second though).
		if err := testing.Sleep(ctx, time.Second*2); err != nil {
			errs = append(errs, err)
		}

		// If the error is halfway through the test, the user won't notice. Put it at the end.
		for _, err := range errs {
			testing.ContextLog(ctx, "Re-showing previous error in video recording: ", err)
		}

		// Call cleanup before the function ends so that we stop getting more frames.
		cleanup()

		end := time.Now()
		testing.ContextLogf(ctx, "starting to encode a %v video recording. This can take a while", end.Sub(startTime))
		canvas := image.NewRGBA(image.Rect(0, 0, int(cc.Width()), int(cc.Height())))
		if err := createVideo(s, &enc, canvas, startTime, time.Now(), cfg); err != nil {
			testing.ContextLog(ctx, "Failed to create video recording. Check above in the log to see if there were other video recording errors: ", err)
		} else {
			testing.ContextLogf(ctx, "Completed encoding %v of video in %v", end.Sub(startTime), time.Since(end))
		}
	}, nil
}
