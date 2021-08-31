// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audionode

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/testing"
)

// ActiveCrasNode returns the active output node.
func ActiveCrasNode(ctx context.Context, cras *audio.Cras) (*audio.CrasNode, error) {
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nodes from cras")
	}

	for _, n := range nodes {
		if n.Active && !n.IsInput {
			return &n, nil
		}
	}
	return nil, errors.New("failed to find active node")
}

// Helper helps to set/get system volume and provides volume related functions.
type Helper struct {
	cras       *audio.Cras
	activeNode *audio.CrasNode
}

// NewVolumeHelper returns a new volume Helper instance.
func NewVolumeHelper(ctx context.Context) (*Helper, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new cras")
	}

	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		return nil, errors.Wrap(err, "failed to wait for output stream")
	}

	node, err := ActiveCrasNode(ctx, cras)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initial active cras node")
	}

	return &Helper{cras, node}, nil
}

// SetVolume sets the volume to the given value.
func (vh *Helper) SetVolume(ctx context.Context, volume int) error {
	return vh.cras.SetOutputNodeVolume(ctx, *vh.activeNode, volume)
}

// ActiveNodeVolume returns the current volume of active node.
func (vh *Helper) ActiveNodeVolume(ctx context.Context) (int, error) {
	node, err := ActiveCrasNode(ctx, vh.cras)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get active cras node")
	}
	if vh.activeNode.ID != node.ID {
		return 0, errors.Errorf("cras active node changed from %+v to %+v during the test", vh.activeNode, node)
	}
	vh.activeNode = node
	return int(vh.activeNode.NodeVolume), nil
}

// VerifyVolumeChanged verifies volume is changed before and after calling doChange().
func (vh *Helper) VerifyVolumeChanged(ctx context.Context, doChange func() error) error {
	prevVolume, err := vh.ActiveNodeVolume(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get volume before calling doChange function")
	}
	if err := doChange(); err != nil {
		return errors.Wrap(err, "failed in calling doChange function")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get volume after doChange function is called"))
		}
		if volume == prevVolume {
			return errors.New("volume not changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for volume change")
	}
	return nil
}

// SetAudioNode sets the expected audio node for audio routing.
func SetAudioNode(ctx context.Context, expectedAudioNode string) (string, string, error) {
	var deviceName, deviceType string
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return deviceName, deviceType, errors.Wrap(err, "failed to create Cras object")
	}
	deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
	if err != nil {
		return deviceName, deviceType, errors.Wrap(err, "failed to get the selected audio device")
	}

	if deviceType != expectedAudioNode {
		testing.ContextLogf(ctx, "%s is not selected, selecting again", expectedAudioNode)
		if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
			return deviceName, deviceType, errors.Wrapf(err, "failed to select active device %s", expectedAudioNode)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			return deviceName, deviceType, errors.Wrap(err, "failed to get the selected audio device")
		}
		if deviceType != expectedAudioNode {
			return deviceName, deviceType, errors.Errorf("failed to select the active device %s", expectedAudioNode)
		}
	}
	return deviceName, deviceType, nil
}
