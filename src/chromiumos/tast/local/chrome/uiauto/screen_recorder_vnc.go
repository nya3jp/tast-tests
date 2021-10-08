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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/kmsvnc"
	"chromiumos/tast/testing"
)

type videoConfig struct {
	fileName        string
	framerate       int
	recordSuccesses bool
}

var defaultConfig = videoConfig{
	fileName:        "recording.webm",
	framerate:       10,
	recordSuccesses: false,
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

// RecordSuccesses ensures that we record video even if the test succeeds.
func RecordSuccesses() func(*videoConfig) {
	return func(c *videoConfig) {
		c.recordSuccesses = true
	}
}

// combineErrors combines multiple errors into a single one.
// Since we have multiple goroutines, and the cleanup can return an error,
// it's quite possible to get an error while trying to clean up another one.
func combineErrors(base error, extra []error) error {
	for _, new := range extra {
		if base == nil {
			base = new
		}
		base = errors.Errorf("While trying to cleanup after error %v, got error: %v", base, new)
	}
	return base
}

// RecordVNCVideo starts recording video from a VNC stream.
// If the recording was unable to start, an error will be returned.
// Otherwise, a function to stop and save the recording will be returned.
// If the stop recording function fails, it will be logged, but as it is
// non-critical to the test itself, the test will still pass.
// Example usage:
// stopRecording, err := RecordVNCVideo(ctx, s, RecordingFramerate(5))
// if err != nil {
// 	handle err
// }
// defer stopRecording()
func RecordVNCVideo(ctx context.Context, s testingState, mods ...func(*videoConfig)) (stopRecording func() error, err error) {
	cfg := defaultConfig
	for _, mod := range mods {
		mod(&cfg)
	}

	kms, err := kmsvnc.NewKmsvnc(ctx)
	if err != nil {
		return nil, err
	}

	cleanups := []func() error{func() error { return kms.Stop(ctx) }}
	// This is basically a fake defer, since we can't use defer because
	// if all goes well, this gets called when we stop recording, not
	// when this function ends.
	cleanup := func() []error {
		var errs []error
		for i := len(cleanups) - 1; i >= 0; i-- {
			if err := cleanups[i](); err != nil {
				errs = append(errs, err)
			}
		}
		return errs
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
		return nil, combineErrors(errors.Wrap(err, "error connecting to localhost:5900"), cleanup())
	}
	// Don't add nc.Close to cleanup, since it gets owned by the vnc.Connect.

	cc, err := vnc.Connect(ctx, nc, ccfg)
	if err != nil {
		if err2 := nc.Close(); err != nil {
			err = combineErrors(err, []error{err2})
		}
		return nil, combineErrors(errors.Wrap(err, "error connecting to vnc protocol"), cleanup())
	}
	cleanups = append(cleanups, func() error { return cc.Close() })

	if err := cc.SetEncodings([]vnc.EncodingType{
		vnc.EncCursorPseudo,
		vnc.EncTight,
	}); err != nil {
		return nil, combineErrors(err, cleanup())
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
				errs = append(errs, errors.Wrap(err, "received error during screen recording - stopping screen recording and continuing test"))
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
						errs = append(errs, errors.Wrap(err, "failed to request an additional frame"))
						return
					}
				}
			}
		}
	}()

	select {
	case err := <-errorCh:
		return nil, combineErrors(combineErrors(err, errs), cleanup())
	case <-recordingStart:
		testing.ContextLog(ctx, "Started screen recording")
	}

	return func() error {
		// Chances are that this is called in a deferred function, so the user
		// won't actually see any error message unless we log it ourselves.
		logError := func(err error) error {
			if err != nil {
				testing.ContextLog(ctx, "Error during video recording: ", err)
			}
			return err
		}

		// VNC stream is slightly behind. Allow a second to capture everything that happened since.
		// Potential room for improvement: start encoding video before this (it's only 1 second though).
		if err := testing.Sleep(ctx, time.Second); err != nil {
			errs = append(errs, err)
		}
		// Call cleanup before the function ends so that we stop getting more frames.
		errs = append(errs, cleanup()...)

		if !cfg.recordSuccesses && !s.HasError() {
			return logError(combineErrors(nil, errs))
		}
		end := time.Now()
		testing.ContextLogf(ctx, "starting to encode a %v video recording. This can take a while", end.Sub(startTime))
		canvas := image.NewRGBA(image.Rect(0, 0, int(cc.Width()), int(cc.Height())))
		if err := createVideo(s, &enc, canvas, startTime, time.Now(), cfg); err != nil {
			errs = append(errs, err)
		} else {
			testing.ContextLogf(ctx, "Completed encoding %v of video in %v", end.Sub(startTime), time.Since(end))
		}
		return logError(combineErrors(nil, errs))
	}, nil
}
