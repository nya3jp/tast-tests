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

var volumeKeys = []string{"volumedown", "volumeup"}

// volumeHelper helps to set/get system volume and verify volume related function.
type volumeHelper struct {
	cras       *audio.Cras
	activeNode *audio.CrasNode
}

func getActiveCrasNode(ctx context.Context, cras *audio.Cras) (*audio.CrasNode, error) {
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		if n.Active && !n.IsInput {
			return &n, nil
		}
	}
	return nil, errors.New("failed to find active node")
}

func newVolumeHelper(ctx context.Context) (*volumeHelper, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, err
	}

	node, err := getActiveCrasNode(ctx, cras)
	if err != nil {
		return nil, err
	}

	return &volumeHelper{cras, node}, nil
}

func (vh *volumeHelper) setVolume(ctx context.Context, volume int) error {
	return vh.cras.SetOutputNodeVolume(ctx, *vh.activeNode, volume)
}

func (vh *volumeHelper) currentVolume(ctx context.Context) (int, error) {
	node, err := getActiveCrasNode(ctx, vh.cras)
	if err != nil {
		return 0, err
	}
	if vh.activeNode.ID != node.ID {
		return 0, errors.Errorf("active node ID changed from %v to %v during the test", vh.activeNode.ID, node.ID)
	}
	vh.activeNode = node
	return int(vh.activeNode.NodeVolume), nil
}

// verifyVolumeChanged verifies volume is changed before and after calling doChange().
func (vh *volumeHelper) verifyVolumeChanged(ctx context.Context, doChange func() error) error {
	prevVolume, err := vh.currentVolume(ctx)
	if err != nil {
		return err
	}
	if err := doChange(); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		volume, err := vh.currentVolume(ctx)
		if err != nil {
			return err
		}
		if volume == prevVolume {
			return errors.New("volume not changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait volume change")
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
	originalVolume, err := vh.currentVolume(ctx)
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
	defer app.Close(ctx)

	restartApp := func() {
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
		{"testRecordVideo", testRecordVideo},
	} {
		if err := tst.testFunc(ctx, app, kb, vh); err != nil {
			s.Errorf("Failed in %v(): %v", tst.name, err)
			restartApp()
		}
	}
}

func testSwitchDeviceMode(ctx context.Context, app *cca.App, kb *input.KeyboardEventWriter, vh *volumeHelper) error {
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		return err
	}

	modeName := func(tablet bool) string {
		if tablet {
			return "tablet"
		}
		return "clamshell"
	}
	for _, tablet := range []bool{false, true} {
		testing.ContextLogf(ctx, "Switch to %v mode", modeName(tablet))
		cleanup, err := app.EnsureTabletModeEnabled(ctx, tablet)
		if err != nil {
			return errors.Wrapf(err, "failed to switch to %v mode", modeName(tablet))
		}
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
		defer cancel()
		defer cleanup(cleanupCtx)

		for _, key := range volumeKeys {
			pressKey := func() error {
				testing.ContextLogf(ctx, "Click %v key in %v mode", key, modeName(tablet))
				return kb.Accel(ctx, key)
			}
			if tablet {
				prevVolume, err := vh.currentVolume(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to get current volume")
				}
				start := time.Now()
				if err := pressKey(); err != nil {
					return errors.Wrapf(err, "failed to press %v key", key)
				}
				dir, err := app.SavedDir(ctx)
				if err != nil {
					return err
				}
				if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
					return err
				}
				if err := app.WaitForState(ctx, "taking", false); err != nil {
					return err
				}
				volume, err := vh.currentVolume(ctx)
				if err != nil {
					return err
				}
				if prevVolume != volume {
					return errors.Errorf("volume changed from %v to %v after shutter", prevVolume, volume)
				}
			} else {
				if err := vh.verifyVolumeChanged(ctx, pressKey); err != nil {
					return errors.Wrapf(err, "volume not changed after press %v key in clamshell mode", err)
				}
			}
		}
	}
	return nil
}

func testRecordVideo(ctx context.Context, app *cca.App, kb *input.KeyboardEventWriter, vh *volumeHelper) error {
	cleanup, err := app.EnsureTabletModeEnabled(ctx, true)
	defer cleanup(ctx)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()
	defer cleanup(cleanupCtx)

	dir, err := app.SavedDir(ctx)
	if err != nil {
		return err
	}
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return err
	}

	for _, key := range volumeKeys {
		prevVolume, err := vh.currentVolume(ctx)
		if err != nil {
			return err
		}

		testing.ContextLogf(ctx, "Press %v key in tablet mode", key)
		if err := kb.Accel(ctx, key); err != nil {
			return errors.Wrapf(err, "failed to press %v key", key)
		}
		if err := app.WaitForState(ctx, "taking", true); err != nil {
			return errors.Wrap(err, "shutter is not started")
		}

		testing.ContextLog(ctx, "Record video for a second")
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return err
		}

		testing.ContextLogf(ctx, "Press %v key in tablet mode", key)
		start := time.Now()
		if err := kb.Accel(ctx, key); err != nil {
			return errors.Wrapf(err, "failed to press %v key", key)
		}
		if _, err := app.WaitForFileSaved(ctx, dir, cca.VideoPattern, start); err != nil {
			return errors.Wrap(err, "cannot find result video")
		}
		if err := app.WaitForState(ctx, "taking", false); err != nil {
			return errors.Wrap(err, "shutter is not ended")
		}

		volume, err := vh.currentVolume(ctx)
		if err != nil {
			return err
		}
		if prevVolume != volume {
			return errors.Errorf("volume changed from %v to %v after shutter", prevVolume, volume)
		}
	}
	return nil
}
