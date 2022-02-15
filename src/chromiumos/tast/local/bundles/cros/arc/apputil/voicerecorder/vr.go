// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package voicerecorder wraps method and constant of voice recorder app for MTBF testing.
package voicerecorder

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
)

// VoiceRecorder creates a struct that contains *apputil.App
type VoiceRecorder struct {
	app *apputil.App
}

const (
	pkgName = "com.media.bestrecorder.audiorecorder"
)

// New returns an instance of VoiceRecorder.
func New(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*VoiceRecorder, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, "Voice Recorder", pkgName)
	if err != nil {
		return nil, err
	}
	return &VoiceRecorder{app: app}, nil
}

// Close closes voice recorder app.
func (vr *VoiceRecorder) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	return vr.app.Close(ctx, cr, hasError, outDir)
}
