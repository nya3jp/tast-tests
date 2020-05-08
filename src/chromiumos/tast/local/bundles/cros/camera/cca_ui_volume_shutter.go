// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIVolumeShutter,
		Desc:         "Verify CCA volume button shutter related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera, "tablet_mode"},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func getActiveCrasNode(ctx context.Context, cras *audio.Cras) (*audio.CrasNode, error) {
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

// volumeHelper helps to set/get system volume and verify volume related function.
type volumeHelper struct {
	cras       *audio.Cras
	activeNode *audio.CrasNode
}

func newVolumeHelper(ctx context.Context) (*volumeHelper, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new cras")
	}

	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		return nil, errors.Wrap(err, "failed to wait for output stream")
	}

	node, err := getActiveCrasNode(ctx, cras)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initial active cras node")
	}

	return &volumeHelper{cras, node}, nil
}

func (vh *volumeHelper) setVolume(ctx context.Context, volume int) error {
	return vh.cras.SetOutputNodeVolume(ctx, *vh.activeNode, volume)
}

func (vh *volumeHelper) refreshVolume(ctx context.Context) (int, error) {
	node, err := getActiveCrasNode(ctx, vh.cras)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get active cras node")
	}
	if vh.activeNode.ID != node.ID {
		return 0, errors.Errorf("active node ID changed from %v to %v during the test", vh.activeNode.ID, node.ID)
	}
	vh.activeNode = node
	return int(vh.activeNode.NodeVolume), nil
}

// verifyVolumeChanged verifies volume is changed before and after calling doChange().
func (vh *volumeHelper) verifyVolumeChanged(ctx context.Context, doChange func() error) error {
	prevVolume, err := vh.refreshVolume(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get volume before doChange()")
	}
	if err := doChange(); err != nil {
		return errors.Wrap(err, "failed in calling doChange()")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		volume, err := vh.refreshVolume(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get volume after doChange()"))
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

func CCAUIVolumeShutter(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	vh, err := newVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.refreshVolume(ctx)
	if err := vh.setVolume(ctx, 50); err != nil {
		s.Fatal("Failed to set volume to 50 percents: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	defer func(ctx context.Context) {
		if err := vh.setVolume(ctx, originalVolume); err != nil {
			s.Fatal("Failed to restore original volume: ", err)
		}
	}(cleanupCtx)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(cleanupCtx)

	restartApp := func(ctx context.Context) {
		s.Log("Restarts CCA")
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *cca.App, *input.KeyboardEventWriter, *volumeHelper) error
	}{
		{"testSwitchDeviceMode", testSwitchDeviceMode},
	} {
		s.Run(ctx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := tst.testFunc(ctx, app, kb, vh); err != nil {
				s.Error("Test failed: ", err)
				restartApp(ctx)
			}
		})
	}
}

func testSwitchDeviceMode(ctx context.Context, app *cca.App, kb *input.KeyboardEventWriter, vh *volumeHelper) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get result saved directory")
	}

	modeName := func(tablet bool) string {
		if tablet {
			return "tablet"
		}
		return "clamshell"
	}

	testDeviceMode := func(tablet bool) error {
		testing.ContextLogf(ctx, "Switch to %v mode", modeName(tablet))
		cleanup, err := app.EnsureTabletModeEnabled(ctx, tablet)
		if err != nil {
			return errors.Wrapf(err, "failed to switch to %v mode", modeName(tablet))
		}
		defer cleanup(cleanupCtx)

		for _, key := range []string{"volumedown", "volumeup"} {
			pressKey := func() error {
				testing.ContextLogf(ctx, "Press %v key in %v mode", key, modeName(tablet))
				return kb.Accel(ctx, key)
			}
			if tablet {
				prevVolume, err := vh.refreshVolume(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to get volume before shutter")
				}
				start := time.Now()
				if err := pressKey(); err != nil {
					return errors.Wrapf(err, "failed to press %v key", key)
				}
				if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
					return errors.Wrap(err, "cannot find captured result file")
				}
				if err := app.WaitForState(ctx, "taking", false); err != nil {
					return errors.Wrap(err, "shutter is not ended")
				}
				volume, err := vh.refreshVolume(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to get volume after shutter")
				}
				if prevVolume != volume {
					return errors.Errorf("volume changed from %v to %v after shutter", prevVolume, volume)
				}
			} else {
				if err := vh.verifyVolumeChanged(ctx, pressKey); err != nil {
					return errors.Wrapf(err, "volume not changed after press %v key in clamshell mode", key)
				}
			}
		}
		return nil
	}

	for _, tablet := range []bool{false, true} {
		if err := testDeviceMode(tablet); err != nil {
			return errors.Wrapf(err, "failed when test in %v mode", modeName(tablet))
		}
	}
	return nil
}
