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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

// AudioFromFile inputs an audio file via active input node and waits for its completion.
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

	return playCmd.Wait()
}

// EnableAloopInput loads and enables Aloop then sets it as active input node.
func EnableAloopInput(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context), error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to cras")
	}

	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load ALSA loopback module")
	}

	if err := uiauto.Combine("activate aloop nodes",
		activateNode(tconn, cras, true),
		activateNode(tconn, cras, false),
	)(ctx); err != nil {
		unload(ctx)
		return nil, errors.New("failed to enable aloop")
	}
	testing.Sleep(ctx, 5*time.Second)
	return unload, nil
}

func activateNode(tconn *chrome.TestConn, cras *audio.Cras, isInput bool) uiauto.Action {
	return func(ctx context.Context) error {
		node, err := findAloopNode(ctx, cras, isInput)
		if err != nil {
			return err
		}
		if err := cras.SetActiveNode(ctx, node); err != nil {
			return errors.New("failed to set Aloop active")
		}

		return testing.Poll(ctx, func(ctx context.Context) error {
			if node, err := findAloopNode(ctx, cras, isInput); err != nil {
				return errors.Wrap(err, "failed to find node")
			} else if !node.Active {
				return errors.New("node is not activated")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}
}

func findAloopNode(ctx context.Context, cras *audio.Cras, isInput bool) (audio.CrasNode, error) {
	var aloopInputNode audio.CrasNode
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			return err
		}

		for _, n := range nodes {
			if n.Type == audio.AloopCrasNodeType && n.IsInput == isInput {
				aloopInputNode = n
				return nil
			}
		}
		return errors.New("Aloop input node does not exist")
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return aloopInputNode, errors.Wrap(err, "failed to find node")
	}
	return aloopInputNode, nil
}
