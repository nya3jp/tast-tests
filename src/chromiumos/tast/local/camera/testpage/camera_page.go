// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testpage provides helper functions to start a simple webpage for
// camera testing.
package testpage

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// CameraWebPage holds all connections to the web page which opens a camera stream.
type CameraWebPage struct {
	pageURL    string
	pageConn   *chrome.Conn
	trackState *chrome.JSObject
}

// CameraFacing for specifying the facingMode constraint.
type CameraFacing string

const (
	// UserFacing corresponds to the "user" facing mode.
	UserFacing CameraFacing = "user"
	// EnvironmentFacing corresponds to the "environment" facing mode.
	EnvironmentFacing = "environment"
)

// ConstraintRange holds the range specifiers for a constraint.
type ConstraintRange struct {
	Min   interface{} `json:"min,omitempty"`
	Max   interface{} `json:"max,omitempty"`
	Ideal interface{} `json:"ideal,omitempty"`
	Exact interface{} `json:"exact,omitempty"`
}

// TrackConstraints holds the collections of constraints for a media track.
type TrackConstraints struct {
	Width      ConstraintRange `json:"width,omitempty"`
	Height     ConstraintRange `json:"height,omitempty"`
	FacingMode ConstraintRange `json:"facingMode,omitempty"`
	FrameRate  ConstraintRange `json:"frameRate,omitempty"`
}

// MediaConstraints holds the TrackConstraints for the audio and video tracks.
// |Audio| is a bool indicating whether to enable audio track. |Video| can be
// a bool value indicating whether to enable camera stream with any usable
// constraints, or a TrackConstraints specifying the exact constraints.
type MediaConstraints struct {
	Audio bool        `json:"audio"`
	Video interface{} `json:"video"`
}

// NewConstraints creates a MediaConstraints based on the given stream
// resolution |width| x |height| and camera facing |facing|.
func NewConstraints(width, height int, facing CameraFacing, framerate float64) *MediaConstraints {
	return &MediaConstraints{
		Audio: false,
		Video: TrackConstraints{
			Width:      ConstraintRange{Exact: width},
			Height:     ConstraintRange{Exact: height},
			FacingMode: ConstraintRange{Exact: facing},
			FrameRate:  ConstraintRange{Exact: framerate}}}
}

// New creates a CameraWebPage from the given base HTTP server URL.
func New(baseURL string) *CameraWebPage {
	return &CameraWebPage{pageURL: baseURL + "/camera_page.html"}
}

// Open starts the video capture with audio disabled and with any usable camera
// stream.
func (w *CameraWebPage) Open(ctx context.Context, cr *chrome.Chrome) error {
	return w.OpenWithConstraints(ctx, cr, &MediaConstraints{Audio: false, Video: true})
}

// OpenWithConstraints starts the video capture with the given MediaConstraints
// |cst|.
func (w *CameraWebPage) OpenWithConstraints(ctx context.Context, cr *chrome.Chrome, cst *MediaConstraints) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var err error
	w.pageConn, err = cr.NewConn(ctx, w.pageURL)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := w.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close web page: ", err)
			}
		}
	}(cleanupCtx)

	var trackState chrome.JSObject
	if err := w.pageConn.Call(ctx, &trackState, "Tast.startStreamWithConstraints", &cst); err != nil {
		return errors.Wrap(err, "failed to setup stream and monitor on the web page")
	}
	w.trackState = &trackState
	return nil
}

// Close releases the media track and closes the test page.
func (w *CameraWebPage) Close(ctx context.Context) (retErr error) {
	if w.trackState != nil {
		var hasEnded bool
		err := w.trackState.Call(ctx, &hasEnded, "function() { return this.hasEnded; }")
		if err != nil {
			retErr = errors.Wrapf(retErr, "failed to check track state: %v", err.Error())
		} else if hasEnded {
			retErr = errors.Wrap(retErr, "failed as media track in web page unexpectedly ended")
		}
		if err := w.trackState.Release(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to release track state: ", err)
			if retErr != nil {
				retErr = errors.Wrapf(retErr, "failed to release track state: %v", err.Error())
			}
		}
		w.trackState = nil
	}
	if w.pageConn != nil {
		if err := w.pageConn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close web page target: ", err)
			if retErr != nil {
				retErr = errors.Wrapf(retErr, "failed to close web page target: %v", err.Error())
			}
		}
		if err := w.pageConn.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close web page connection: ", err)
			if retErr != nil {
				retErr = errors.Wrapf(retErr, "failed to close web page connection: %v", err.Error())
			}
		}
		w.pageConn = nil
	}
	return
}
