// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// AudioFromFile inputs an audio file via active input node.
func AudioFromFile(ctx context.Context, audioFilePath string) error {
	audioInput := audio.TestRawData{
		Path:          audioFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
	}

	// Playback function by CRAS.
	playCmd := crastestclient.PlaybackFileCommand(
		ctx, audioInput.Path,
		audioInput.Duration,
		audioInput.Channels,
		audioInput.Rate)
	playCmd.Start()

	// Wait a short time to make sure playback command is working.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		if waitErr := playCmd.Wait(); waitErr != nil {
			return errors.Wrap(waitErr, "playback did not finish in time")
		}
		return errors.Wrap(err, "failed to wait 1sec after playback command")
	}
	return nil
}

// EnableALoopInput loads and enables Aloop then sets it as active input node.
func EnableALoopInput(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context), error) {
	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load ALSA loopback module")
	}

	// Activate aloop input node.
	if err := func() error {
		cras, err := audio.NewCras(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to connect to cras")
		}

		// Wait until the Aloop input node to be available.
		aloopInputNode, err := findAloopInputNode(ctx, cras)
		if err != nil {
			return errors.New("failed to find aloop input node")
		}

		if err := cras.SetActiveNode(ctx, aloopInputNode); err != nil {
			return errors.New("failed to set Aloop input active")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			aloopInputNode, err := findAloopInputNode(ctx, cras)
			if err != nil {
				return errors.New("failed to find aloop input node")
			}
			if !aloopInputNode.Active {
				return errors.New("aloop input is not active")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.New("failed to find aloop input node")
		}
		return nil
	}(); err != nil {
		// Unload ALSA loopback if any following setups failed.
		unload(ctx)
		return nil, err
	}

	return func(ctx context.Context) {
		unload(ctx)
	}, nil
}

func findAloopInputNode(ctx context.Context, c *audio.Cras) (audio.CrasNode, error) {
	var aloopInputNode audio.CrasNode
	err := testing.Poll(ctx, func(ctx context.Context) error {
		nodes, err := c.GetNodes(ctx)
		if err != nil {
			return err
		}

		for _, n := range nodes {
			if n.Type == audio.AloopCrasNodeType && n.IsInput {
				aloopInputNode = n
				return nil
			}
		}
		return errors.New("aloop input node does not exist")
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	return aloopInputNode, err
}
