// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// deviceMode represents the different device modes used in this test.
type deviceMode int

const (
	// deviceModeClamshell represents the device in clamshell mode.
	deviceModeClamshell deviceMode = iota
	// deviceModeTabletLandscpape represents the device in tablet mode with landscape orientation.
	deviceModeTabletLandscape
	// deviceModeTabletLandscpape represents the device in tablet mode with portrait orientation.
	deviceModeTabletPortrait
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HWOverlayPortrait,
		Desc: "Checks that hardware overlay works with ARC applications",
		// TODO(ricardoq): enable test once the the bug that fixes hardware overlay gets landed. See: http://b/120557146
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"hw_overlay", "android", "android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

// HWOverlayPortrait checks whether ARC apps use hw overlay, instead of being composited by the renderer.
// There 3 ways to check that:
//
// 1) By parsing /sys/kernel/debug/dri/?/state.
// It seems to be the "correct" way to do it, since the 'state' file shows all the crtc buffers being used,
// like the primary, mouse and overlays buffers.
// The drawback is that when the device is in tablet mode, the overlay and mouse buffers are not used.
// This happens because the mouse is not used in tablet mode. And since ARC applications are in fullscreen mode
// there is not need to have both a primary and overlay buffers.
// Parsing GPU-specific files like the /sys/kernel/debug/dri/0/i915_display_info doesn't help either.
//
// 2) By using the screenshot CLI.
// The screenshot CLI takes screenshots from the primary buffer only. So any mouse or overlay buffers are ignored.
// This "bug" is actually a "feature". Overlay buffers will appear as totally black, and we can detect whether overlay is
// being used by counting the black pixels within ARC bounds.
// But it has the same drawback as 1). When the device is in tablet mode, only the primary buffer will be used,
// and the screenshot will actually include the ARC content.
//
// 3) By enabling --tint-gl-composited-content.
// When --tint-gl-composited-content is enabled, all composited buffers will be tinted red. That means that
// hardware overlay buffers won't be tinted.
// So we can check that overlay is working by taking an screenshot and verifying that there are no tinted pixels
// inside the ARC window. The screenshot should be taken using the Chrome JS API, and not the screenshot CLI, since
// Chrome JS API composites all the available crtc buffers.
//
// So far option 3) is the only one that works both on tablet and clamshell mode for ARC apps.
func HWOverlayPortrait(ctx context.Context, s *testing.State) {
	// Should not fail, since it is guaranteed by "hw_overlay" SoftwareDeps.
	if !supportsHardwareOverlay() {
		s.Fatal("Hardware overlay not supported. Perhaps 'hw_overlay' USE property added to the incorrect board?")
	}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--tint-gl-composited-content"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}
	s.Logf("Settings bounds: %v", bounds)

	// err = testing.Poll(ctx, func(ctx context.Context) error {
	// 	s.Log("Polling...")
	// 	postOverlays, err := activeHWOverlays(ctx, overlayFile)
	// 	if err != nil {
	// 		s.Fatal("Failed to get hw overlays: ", err)
	// 	}
	// 	if len(postOverlays) != len(preOverlays)+1 {
	// 		return errors.New("hw overlay not ready")
	// 	}
	// 	s.Logf("post-overlays: %v", postOverlays)
	// 	return nil
	// }, &testing.PollOptions{Timeout: 5 * time.Second})
	// if err != nil {
	// 	s.Fatal("ARC++ application not promoted to hw overlay: ", err)
	// }

	for _, entry := range []struct {
		mode deviceMode
		desc string
	}{
		{deviceModeClamshell, "clamshell"},
		{deviceModeTabletLandscape, "tablet + landscape"},
		{deviceModeTabletPortrait, "tablet + portrait"},
	} {
		s.Log("Testing hardware overlay in ", entry.desc)

		// if err := screenshot.CaptureChrome(ctx, cr, "/tmp/mama.png"); err != nil {
		// 	s.Error("Could not take screenshot: ", err)
		// }

		if err := setDeviceMode(ctx, cr, entry.mode); err != nil {
			s.Error("Failed to set device mode: ", err)
		}
		// select {
		// case <-time.After(1 * time.Second):
		// case <-ctx.Done():
		// 	s.Error("Timeout: ", err)
		// }
		if err := isTabletModeEnabled(ctx, cr); err != nil {
			s.Error("Failed to get tablet mode: ", err)
		}

		// if err := verifyHWOverlay(ctx, s); err != nil {
		// 	s.Error("Hardware overlay not being used: ", err)
		// }

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			s.Error("Timeout: ", err)
		}

	}
	// pm, err := power.NewPowerManager(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create a PowerManager: ", err)
	// }

	// sw, err := pm.GetSwitchStates(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to get switch states: ", err)
	// }

	// if sw.TabletMode != nil && *sw.TabletMode == pmpb.SwitchStates_OFF {
	// 	s.Log("This test is only valid when the device is in Tablet mode. Skipping test")
	// 	return
	// }

}

func setDeviceMode(ctx context.Context, cr *chrome.Chrome, mode deviceMode) error {
	tabletEnabled := true
	if mode == deviceModeClamshell {
		tabletEnabled = false
	}

	if err := setTabletModeEnabled(ctx, cr, tabletEnabled); err != nil {
		return err
	}
	return nil
}

func verifyHWOverlay(ctx context.Context, s *testing.State) error {
	return nil
}

// supportsHardwareOverlay returns true if hardware overlay is supported on the device. false otherwise.
func supportsHardwareOverlay() bool {
	// The 'state' file is the one that has the HW overlay state. Depending on the device, it
	// could be either in the .../dri/0/ or .../dri/1/ directories.
	driDebugFiles := []string{"/sys/kernel/debug/dri/0/state", "/sys/kernel/debug/dri/1/state"}
	for i := 0; i < len(driDebugFiles); i++ {
		_, err := os.Stat(driDebugFiles[i])
		if err == nil {
			return true
		}
	}
	return false
}

func setTabletModeEnabled(ctx context.Context, cr *chrome.Chrome, enabled bool) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	strEnabled := "true"
	if !enabled {
		strEnabled = "false"
	}

	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
	       chrome.autotestPrivate.setTabletModeEnabled(`+strEnabled+`, () => {
		     if (chrome.runtime.lastError === undefined) {
			   resolve();
		     } else {
			   reject(chrome.runtime.lastError.message);
		     }
		   });
	     })`, nil); err != nil {
		return errors.Wrap(err, "running autotestPrivate.setTabletModeEnabled failed")
	}
	return nil
}

func isTabletModeEnabled(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	var isEnabled bool
	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.isTabletModeEnabled((isEnabled) => {
			  if (chrome.runtime.lastError === undefined) {
				resolve(isEnabled);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, &isEnabled); err != nil {
		return err
	}
	testing.ContextLog(ctx, "*** Enabled = ", isEnabled)
	return nil
}
