// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// AloopCrasNodeType defines CrasNode type for ALSA loopback.
const AloopCrasNodeType = "ALSA_LOOPBACK"

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

func findDevice(ctx context.Context, devices []CrasNode, isInput bool) (CrasNode, error) {
	for _, n := range devices {
		if n.Type == AloopCrasNodeType && n.IsInput == isInput {
			return n, nil
		}
	}
	return CrasNode{}, errors.Errorf("cannot find device with type=%s and isInput=%v", AloopCrasNodeType, isInput)
}

// SetupLoopback sets the playback and capture device using alsa loopback device.
func SetupLoopback(ctx context.Context) error {
	cras, err := NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cras")
	}

	var playbackFound, captureFound bool
	checkLoopbackNode := func(n *CrasNode) bool {
		if n.Type != AloopCrasNodeType {
			return false
		}
		if n.IsInput {
			captureFound = true
		} else {
			playbackFound = true
		}
		return captureFound && playbackFound
	}

	if err = cras.WaitForDeviceUntil(ctx, checkLoopbackNode, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for loopback devices")
	}

	audioDevices, err := cras.GetNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get nodes")
	}

	playbackDevice, err := findDevice(ctx, audioDevices, false)
	if err != nil {
		return errors.Wrap(err, "failed to find audio device")
	}

	captureDevice, err := findDevice(ctx, audioDevices, true)
	if err != nil {
		return errors.Wrap(err, "failed to find audio device")
	}

	cras.SetActiveNode(ctx, playbackDevice)
	cras.SetActiveNode(ctx, captureDevice)
	cras.SetOutputNodeVolume(ctx, playbackDevice, 100)

	return nil
}
