// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
)

// SetupLoopbackDevice setups ALSA loopback (aloop) module and select the loopback devices
// as the output and input.
func SetupLoopbackDevice(ctx context.Context, cr *chrome.Chrome) (cleanup func(context.Context), err error) {
	timeForCleanUp := 10 * time.Second
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, timeForCleanUp)
	defer cancel()

	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load ALSA loopback module")
	}

	cleanup = func(ctx context.Context) {
		// Wait for no stream before unloading aloop as unloading while there is a stream
		// will cause the stream in ARC to be in an invalid state.
		_ = crastestclient.WaitForNoStream(ctx, 5*time.Second)
		unload(ctx)
	}

	if err := audio.SetupLoopback(ctx, cr); err != nil {
		cleanup(ctxForCleanUp)
		return nil, errors.Wrap(err, "failed to setup loopback")
	}

	return cleanup, nil
}
