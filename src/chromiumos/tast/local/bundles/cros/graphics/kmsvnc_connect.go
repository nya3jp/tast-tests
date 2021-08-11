// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KmsvncConnect,
		Desc:         "Connects to kmsvnc server and verifies server parameters",
		Contacts:     []string{"shaochuan@chromium.org", "uekawa@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
	})
}

// KmsvncConnect launches the kmsvnc server, connects to it, and verifies server parameters.
func KmsvncConnect(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	k, err := graphics.NewKmsvnc(ctx)
	if err != nil {
		s.Fatal("Failed to start kmsvnc: ", err)
	}
	defer k.Stop(ctx)

	serverInit, err := k.Connect(ctx)
	if err != nil {
		s.Fatal("Failed to connect to kmsvnc server: ", err)
	}

	// Verify server parameters.
	gotW, gotH := int(serverInit.FramebufferWidth), int(serverInit.FramebufferHeight)
	wantW, wantH, err := findDisplayWidthHeight(ctx, cr)
	if err != nil {
		s.Error("Failed to find primary display size: ", err)
	}

	if wantW%4 != 0 {
		s.Logf("Screen width %d will be padded to be a multiple of 4", wantW)
		wantW += (4 - (wantW % 4))
	}
	if gotW != wantW || gotH != wantH {
		s.Errorf("Unexpected framebuffer size, got %dx%d, want %dx%d", gotW, gotH, wantW, wantH)
	}

	got := serverInit.PixelFormat
	want := []byte{
		0x20,       // bits-per-pixel
		0x20,       // depth
		0x00,       // big-endian-flag
		0xff,       // true-color-flag
		0x00, 0xff, // red-max
		0x00, 0xff, // green-max
		0x00, 0xff, // blue-max
		0x10, // red-shift
		0x08, // green-shift
		0x00, // blue-shift
	}
	if !cmp.Equal(got, want) {
		s.Errorf("Unexpected pixel format, got %v, want %v", got, want)
	}
}

// findDisplayWidthHeight returns the width/height of the primary display, which should match the VNC framebuffer size.
func findDisplayWidthHeight(ctx context.Context, cr *chrome.Chrome) (int, int, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to connect to test API")
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get primary display info")
	}

	dm, err := info.GetSelectedMode()
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get selected display mode")
	}
	// TODO(b/177965296): handle rotation on tablet devices?
	return dm.WidthInNativePixels, dm.HeightInNativePixels, nil
}
