// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

// LoadAloop loads snd-aloop module on kernel. A deferred call to the returned
// unloadAloop function to unload snd-aloop should be scheduled by the caller if
// err is non-nil.
func LoadAloop(ctx context.Context) (func(ctx context.Context), error) {
	const aloopModuleName = "snd-aloop"

	if err := testexec.CommandContext(ctx, "modprobe", aloopModuleName).Run(testexec.DumpLogOnError); err != nil {
		return nil, err
	}

	return func(ctx context.Context) {
		// Process cras should be stopped first, otherwise snd-aloop would not be unloaded successfully.
		if err := testexec.CommandContext(ctx, "stop", "cras").Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to stop cras: ", err)
			return
		}
		if err := testexec.CommandContext(ctx, "modprobe", "-r", aloopModuleName).Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLogf(ctx, "Failed to unload %s: %v", aloopModuleName, err)
		}
		if err := testexec.CommandContext(ctx, "start", "cras").Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to start cras: ", err)
		}
	}, nil
}

// SetupLoopback sets the playback and capture nodes to the ALSA loopback via the Quick Settings UI .
func SetupLoopback(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	timeForCleanUp := 5 * time.Second
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, timeForCleanUp)
	defer cancel()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show the quicksettings to select playback node")
	}
	defer func() {
		if err := quicksettings.Hide(ctxForCleanUp, tconn); err != nil {
			testing.ContextLog(ctx, "Failed to hide the quicksettings on defer: ", err)
		}
	}()
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Playback"); err != nil {
		return errors.Wrap(err, "failed to select ALSA loopback output")
	}

	// After selecting Loopback Playback, SelectAudioOption() sometimes detected that audio setting
	// is still opened while it is actually fading out, and failed to select Loopback Capture.
	// Call Hide() and Show() to reset the quicksettings menu first.
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to hide the quicksettings before show")
	}
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show the quicksettings to select capture node")
	}
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Capture"); err != nil {
		return errors.Wrap(err, "failed to select ALSA loopback input")
	}

	return nil
}
