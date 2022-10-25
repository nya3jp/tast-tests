// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

// LoadFakeUSBSoundcard loads snd-dummy module on kernel. A deferred call to the returned
// unLoadFakeUSBSoundcard function to unload snd-dummy should be scheduled by the caller if
// err is nil.
func LoadFakeUSBSoundcard(ctx context.Context, numberOfVolumeSteps int) (func(ctx context.Context), error) {
	const moduleName = "snd-dummy"
	mixerVolumeLevelMin := fmt.Sprintf("mixer_volume_level_min=%d", 0)
	mixerVolumeLevelMax := fmt.Sprintf("mixer_volume_level_max=%d", numberOfVolumeSteps)
	if err := testexec.CommandContext(ctx, "modprobe", moduleName, mixerVolumeLevelMin, mixerVolumeLevelMax).Run(testexec.DumpLogOnError); err != nil {
		return nil, err
	}

	return func(ctx context.Context) {
		// Process cras should be stopped first, otherwise snd-dummy would not be unloaded successfully.
		if err := testexec.CommandContext(ctx, "stop", "cras").Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to stop cras: ", err)
			return
		}
		if err := testexec.CommandContext(ctx, "modprobe", "-r", moduleName).Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLogf(ctx, "Failed to unload %s: %v", moduleName, err)
		}
		if err := testexec.CommandContext(ctx, "start", "cras").Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to start cras: ", err)
		}
	}, nil
}

// SetupFakeUSBNode sets the playback nodes to the ALSA Dummy via the Quick Settings UI.
func SetupFakeUSBNode(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show the quicksettings to select playback node")
	}
	defer func() {
		if err := quicksettings.Hide(ctxForCleanUp, tconn); err != nil {
			testing.ContextLog(ctxForCleanUp, "Failed to hide the quicksettings on defer: ", err)
		}
	}()
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Dummy Playback (USB)"); err != nil {
		return errors.Wrap(err, "failed to select ALSA Dummy Playback")
	}

	return nil
}
