// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"io/ioutil"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

type hdrExternalMonitorParams struct {
	fileName            string
	makeExternalPrimary bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HdrExternalMonitor,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that HDR videos played on external monitors are played correctly",
		Contacts: []string{
			"jshargo@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "hdr_external_primary",
				Val: hdrExternalMonitorParams{
					fileName:            "peru.8k.cut.hdr.vp9.webm",
					makeExternalPrimary: true,
				},
				ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
				ExtraData: []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
				Fixture:   "chromeVideo",
			},
			{
				Name: "hdr_internal_primary",
				Val: hdrExternalMonitorParams{
					fileName:            "peru.8k.cut.hdr.vp9.webm",
					makeExternalPrimary: false,
				},
				ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
				ExtraData: []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
				Fixture:   "chromeVideo",
			},
		},
	})
}

func HdrExternalMonitor(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(hdrExternalMonitorParams)

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open test connection: ", err)
	}

	///////////////////////////////////////////////////////////////////////////
	//// Open a browser so we can move it.
	///////////////////////////////////////////////////////////////////////////
	if conn, err := cr.NewConn(ctx, "about:blank"); err == nil {
		defer conn.CloseTarget(ctx)
	} else {
		s.Fatal("Unable to create window: ", err)
	}

	///////////////////////////////////////////////////////////////////////////
	//// Helper functions!
	///////////////////////////////////////////////////////////////////////////
	getInternalDisplay := func() *display.Info {
		internalDisplay, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find internal display: ", err)
		}
		return internalDisplay
	}
	// TODO(jshargo): this should support chameleon for testing in CQ.
	getExternalDisplay := func() *display.Info {
		externalDisplay, err := display.FindInfo(ctx, tconn, func(d *display.Info) bool {
			return !d.IsInternal
		})
		if err != nil {
			s.Fatal("Could not find external display: ", err)
		}
		return externalDisplay
	}
	setPrimary := func(info *display.Info) error {
		isPrimary := true
		if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{IsPrimary: &isPrimary}); err != nil {
			return err
		}

		// Setting the display properties can take a while, so poll until it's done
		return testing.Poll(ctx, func(ctx context.Context) error {
			newInfo, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				return err
			}
			if newInfo.ID != info.ID {
				return errors.New("Not ready yet")
			}
			return nil
		}, nil)
	}
	defer setPrimary(getInternalDisplay())

	moveToExternal := func() {
		externalDisplay := getExternalDisplay()

		window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return true })
		if err != nil {
			s.Fatal("Unable to find and move window: ", err)
		}

		// The window needs to be in normal state to be moved.
		if _, err := ash.SetWindowState(ctx, tconn, window.ID, ash.WMEventNormal, true); err != nil {
			s.Fatal("Unable to update window state: ", err)
		}

		bounds := externalDisplay.Bounds.WithInset(100, 100)
		_, newDisplayID, err := ash.SetWindowBounds(ctx, tconn, window.ID, bounds, externalDisplay.ID)
		if err != nil {
			s.Fatal("Unable to update display: ", err)
		} else if newDisplayID != externalDisplay.ID {
			s.Fatal("ash.SetWithBounds did not move the window :(")
		}
	}

	///////////////////////////////////////////////////////////////////////////
	//// Run the test!
	///////////////////////////////////////////////////////////////////////////

	// TODO(jshargo): verify that the external display is HDR (or set it via chameleon)

	var primaryDisplay *display.Info
	if testOpt.makeExternalPrimary {
		primaryDisplay = getExternalDisplay()
	} else {
		primaryDisplay = getInternalDisplay()
	}

	if err := setPrimary(primaryDisplay); err != nil {
		s.Fatal("Unable to set internal display as primary: ", err)
	}
	moveToExternal()

	if err := play.TestPlay(ctx, s, cs, cr, testOpt.fileName, play.NormalVideo, play.NoVerifyHWAcceleratorUsed, false); err != nil {
		s.Fatal("TestPlay failed: ", err)
	}

	// TODO(jshargo): I think we can turn this into an actual test case by
	// looking at the media internals instead (since this file has different
	// formats on different devices).
	if b, err := ioutil.ReadFile("/sys/kernel/debug/dri/0/i915_display_info"); err == nil {
		out := path.Join(s.OutDir(), "i915_display_info")
		ioutil.WriteFile(out, b, 0664)
	} else {
		s.Fatal("Unable to read display info: ", err)
	}
}
