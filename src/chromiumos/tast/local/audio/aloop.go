// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// AloopCrasNodeType defines CrasNode type for ALSA loopback.
const AloopCrasNodeType = "ALSA_LOOPBACK"

// LoadAloop loads snd-aloop module on kernel. A deferred call to the returned
// unloadAloop function to unload snd-aloop should be scheduled by the caller if
// err is non-nil.
func LoadAloop(ctx context.Context) (unloadAloop func(ctx context.Context), err error) {
	const aloopModuleName = "snd-aloop"

	err = testexec.CommandContext(ctx, "modprobe", aloopModuleName).Run(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	unloadAloop = func(ctx context.Context) {
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
	}
	return unloadAloop, nil
}
