// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// deviceMode represents the different device modes used in this test.
type deviceMode int

func init() {
	testing.AddTest(&testing.Test{
		Func: HWOverlayPortrait,
		Desc: "Checks that hardware overlay works with ARC applications",
		// TODO(ricardoq): enable test once the the bug that fixes hardware overlay gets landed. See: http://b/120557146
		Attr:         []string{"disabled", "informational"},
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

	// TODO(ricardoq): Add clamshell mode tests as well.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(
		// []string{"--force-tablet-mode=touch_view", "--tint-gl-composited-content"}))
		[]string{"--tint-gl-composited-content"}))
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

	deviceID, err := internalDisplayID(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get internal display ID: ", err)
	}

	// Leave Chromebook in reasonable state.
	defer func() { setRotation(ctx, cr, deviceID, rotation0) }()

	for _, entry := range []struct {
		rot  rotation
		desc string
	}{
		{rotation0, "0"},
		{rotation90, "90"},
		{rotation180, "180"},
		{rotation270, "270"},
	} {
		s.Log("Testing hardware overlay in rotation:", entry.desc)

		if err := setRotation(ctx, cr, deviceID, entry.rot); err != nil {
			s.Error("Failed to set rotation: ", err)
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			s.Error("Timeout: ", err)
		}

		// img, err := grabScreenshot(ctx, cr, fmt.Sprintf("%s/screenshot-rot-%d.png", s.OutDir(), entry.rot))
		img, err := grabScreenshot2(ctx, fmt.Sprintf("/tmp/screenshot-rot-%d.png", entry.rot))
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}

		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get activity bounds: ", err)
		}

		subImage := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Right-bounds.Left, bounds.Bottom-bounds.Top))

		whitePixels := countWhitePixels(subImage)
		rect := subImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := whitePixels * 100 / totalPixels
		s.Logf("White pixels = %d / %d (%d%%)", whitePixels, totalPixels, percent)

	}
}

func verifyHWOverlay(ctx context.Context, s *testing.State, path string) error {
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

// internalDisplayID returns the display ID of the internal display.
func internalDisplayID(ctx context.Context, cr *chrome.Chrome) (id string, err error) {
	displays, err := displaysInfo(ctx, cr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get displays info")
	}

	for _, d := range displays {
		val, ok := d["isInternal"]
		if !ok {
			return "", errors.New("could not find 'isInternal' property")
		}

		isInternal := val.(bool)
		if !isInternal {
			continue
		}

		val, ok = d["id"]
		if !ok {
			return "", errors.New("could not find 'id' property")
		}
		return val.(string), nil
	}
	return "", errors.New("could not found internal id")
}

// displaysInfo requests the information for all attached display devices.
// info is the value returned from JS API: chrome.system.display.getInfo()
// See: https://developer.chrome.com/apps/system_display#method-getInfo
func displaysInfo(ctx context.Context, cr *chrome.Chrome) (info []map[string]interface{}, err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.system.display.getInfo({}, (info) => {
			  if (chrome.runtime.lastError === undefined) {
				resolve(info);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// rotation represents the rotation angles: 0, 90, 180 or 270.
type rotation int

const (
	// rotation0 represents a rotation of 0 degrees.
	rotation0 rotation = iota
	// rotation90 represents a rotation of 90 degrees.
	rotation90
	// rotation represents a rotation of 180 degrees.
	rotation180
	// rotation270 represents a rotation of 270 degrees.
	rotation270
)

// setRotation sets the rotation for the display specified by id.
// The rotation is set clockwise. r is the new rotation angle.
func setRotation(ctx context.Context, cr *chrome.Chrome, id string, r rotation) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	var rot string
	switch r {
	case rotation0:
		rot = "0"
	case rotation90:
		rot = "90"
	case rotation180:
		rot = "180"
	case rotation270:
		rot = "270"
	}

	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.system.display.setDisplayProperties("`+id+`", {"rotation":`+rot+`}, () => {
			  if (chrome.runtime.lastError === undefined) {
				resolve();
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, nil); err != nil {
		return err
	}
	return nil
}

// countWhitePixels returns how many white pixels are contained in image.
func countWhitePixels(image image.Image) int {
	// TODO(ricardoq): At least on Eve, Nocturne, Caroline, Kevin and Dru the color
	// that we are looking for is RGBA(255,255,255,255). But it might be possible that
	// on certain devices the color is slightly different. In that case we should
	// adjust the colorMaxDiff.
	const colorMaxDiff = 0
	white := color.RGBA{255, 255, 255, 255}
	rect := image.Bounds()
	whitePixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), white, colorMaxDiff) {
				whitePixels++
			}
		}
	}
	return whitePixels
}

// grabScreenshot creates a screenshot in path, and returns an image.Image.
func grabScreenshot2(ctx context.Context, path string) (image.Image, error) {
	if err := screenshot.Capture(ctx, path); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	fd, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer fd.Close()

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}
