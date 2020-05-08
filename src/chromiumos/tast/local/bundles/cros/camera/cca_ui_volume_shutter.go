// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

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

// volumeTracker helps to set and track system volume.
type volumeTracker struct {
	cras       *audio.Cras
	activeNode *audio.CrasNode
}

func newVolumeTracker(ctx context.Context) (*volumeTracker, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, err
	}
	return &volumeTracker{cras, nil}, nil
}

func (vt *volumeTracker) setVolume(ctx context.Context, volume int) error {
	return vt.cras.SetOutputNodeVolume(ctx, *vt.activeNode, volume)
}

func (vt *volumeTracker) getVolume(ctx context.Context) (int, error) {
	nodes, err := vt.cras.GetNodes(ctx)
	if err != nil {
		return 0, err
	}

	for _, n := range nodes {
		if n.Active && !n.IsInput {
			if vt.activeNode != nil && vt.activeNode.ID != n.ID {
				return 0, errors.Errorf("active node ID changed from %v to %v during the test", vt.activeNode.ID, n.ID)
			}
			vt.activeNode = &n
			return int(vt.activeNode.NodeVolume), nil
		}
	}
	return 0, errors.New("failed to find active node")
}

// verifyVolumeChanged will call doChange() and wait until volume changed.
func (vt *volumeTracker) verifyVolumeChanged(ctx context.Context, doChange func() error) error {
	prevVolume, err := vt.getVolume(ctx)
	if err != nil {
		return err
	}
	if err := doChange(); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		volume, err := vt.getVolume(ctx)
		if err != nil {
			return err
		}
		if volume == prevVolume {
			return errors.New("volume not changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		return errors.Errorf("failed to wait volume change %s", err)
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

	vt, err := newVolumeTracker(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeTracker: ", err)
	}
	originalVolume, err := vt.getVolume(ctx)
	if err := vt.setVolume(ctx, 50); err != nil {
		s.Fatal("Failed to set volume to 50 percents: ", err)
	}
	defer (func() {
		if err := vt.setVolume(ctx, originalVolume); err != nil {
			s.Fatal("Failed to restore original volume: ", err)
		}
	})()

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
		testFunc func(context.Context, *cca.App, *input.KeyboardEventWriter, *volumeTracker) error
	}{
		{"testSwitchDeviceMode", testSwitchDeviceMode},
	} {
		if err := tst.testFunc(ctx, app, kb, vt); err != nil {
			s.Errorf("Failed in %v(): %v", tst.name, err)
			restartApp()
		}
	}
}

func testSwitchDeviceMode(ctx context.Context, app *cca.App, kb *input.KeyboardEventWriter, vt *volumeTracker) error {
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
		defer cleanup(ctx)

		for _, key := range volumeKeys {
			pressKey := func() error {
				testing.ContextLogf(ctx, "Click %v key in %v mode", key, modeName(tablet))
				return kb.Accel(ctx, key)
			}
			if tablet {
				prevVolume, err := vt.getVolume(ctx)
				if err != nil {
					return err
				}
				start := time.Now()
				if err := pressKey(); err != nil {
					return err
				}
				dir, err := app.GetSavedDir(ctx)
				if err != nil {
					return err
				}
				if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
					return err
				}
				if err := app.WaitForState(ctx, "taking", false); err != nil {
					return err
				}
				volume, err := vt.getVolume(ctx)
				if err != nil {
					return err
				}
				if prevVolume != volume {
					return errors.Errorf("volume changed from %v to %v after shutter", prevVolume, volume)
				}
			} else {
				if err := vt.verifyVolumeChanged(ctx, pressKey); err != nil {
					return errors.Wrapf(err, "volume not changed after press %v key in clamshell mode", err)
				}
			}
		}
	}
	return nil
}
