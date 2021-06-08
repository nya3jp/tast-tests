// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPTZ,
		Desc:         "Opens CCA and verifies the PTZ functionality",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome"},
		Data:         []string{"cca_ui.js"},
	})
}

const (
	y4mWidth      = 1280
	y4mHeight     = 720
	patternWidth  = 101
	patternHeight = 101
)

// preparePattern prepares fake preview y4m file.
func preparePattern() (_ string, retErr error) {
	file, err := ioutil.TempFile(os.TempDir(), "*.y4m")
	if err != nil {
		return "", err
	}
	defer func() {
		if retErr != nil {
			os.Remove(file.Name())
		}
	}()

	header := fmt.Sprintf("YUVMPEG2 W%d H%d F30:1 Ip A0:0 C420jpeg\nFRAME\n", y4mWidth, y4mHeight)
	if _, err := file.WriteString(header); err != nil {
		return "", errors.Wrap(err, "failed to write header of temp y4m")
	}

	// White background.
	const (
		bgY = 255
		bgU = 128
		bgV = 128
	)

	// Y plane.
	yp := make([][]byte, y4mHeight)
	for y := range yp {
		yp[y] = make([]byte, y4mWidth)
		for x := range yp[y] {
			yp[y][x] = bgY
		}
	}

	// Draws black square pattern at the center.
	cy := y4mHeight / 2
	cx := y4mWidth / 2
	for dy := -patternHeight / 2; dy <= patternHeight/2; dy++ {
		for dx := -patternWidth / 2; dx <= patternWidth/2; dx++ {
			yp[cy+dy][cx+dx] = 0
		}
	}

	for _, bs := range yp {
		if _, err := file.Write(bs); err != nil {
			return "", errors.Wrap(err, "failed to write Y plane of temp y4m")
		}
	}

	// U plane.
	up := make([]byte, y4mWidth*y4mHeight/4)
	for x := 0; x < len(up); x++ {
		up[x] = bgU
	}
	if _, err := file.Write(up); err != nil {
		return "", errors.Wrap(err, "failed to write U plane of temp y4m")
	}

	// V plane.
	vp := make([]byte, y4mWidth*y4mHeight/4)
	for x := 0; x < len(vp); x++ {
		vp[x] = bgV
	}
	if _, err := file.Write(vp); err != nil {
		return "", errors.Wrap(err, "failed to write V plane of temp y4m")
	}

	if err := os.Chmod(file.Name(), 0644); err != nil {
		return "", err
	}

	return file.Name(), nil
}

// findPattern finds the region where the pattern resides.
func findPattern(ctx context.Context, app *cca.App) (*image.Rectangle, error) {
	frame, err := app.PreviewFrame(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get preview frame")
	}
	defer frame.Release(ctx)

	// Find the coordinates of top-left corner.
	minPt, err := frame.Find(ctx, &cca.FirstBlack)
	if err != nil {
		return nil, err
	}

	// Find the coordinates of bottom-right corner.
	maxPt, err := frame.Find(ctx, &cca.LastBlack)
	if err != nil {
		return nil, err
	}

	return &image.Rectangle{*minPt, *maxPt}, nil
}

// calShift checks the size and calculates the x-y shift between |r| and |r2|.
func calShift(r, r2 *image.Rectangle) (*image.Point, error) {
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}

	// The pattern should only shift without resizing.
	sz := r.Size()
	sz2 := r2.Size()

	// Do all comparisons with 1px precision tolerance introduced by fake
	// file VCD bilinear resizing implementation.
	const precision = 1
	if abs(sz.X-sz2.X) > precision {
		return nil, errors.Errorf("inconsistent width, got %v; want %v", sz2.X, sz.X)
	}
	if abs(sz.Y-sz2.Y) > precision {
		return nil, errors.Errorf("inconsistent height, got %v; want %v", sz2.Y, sz.Y)
	}
	return &image.Point{r2.Min.X - r.Min.X, r2.Min.Y - r.Min.Y}, nil
}

type ptzControl struct {
	// ui is the UI toggled for moving preview in one of PTZ direction.
	ui *cca.UIComponent
	// testFunc tests pattern before and after ptz control applied moving in the target direction.
	testFunc func(r, r2 *image.Rectangle) (bool, error)
}

var (
	panLeft = ptzControl{&cca.PanLeftButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X < 0 && shift.Y == 0, nil
	}}
	panRight = ptzControl{&cca.PanRightButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X > 0 && shift.Y == 0, nil
	}}
	tiltDown = ptzControl{&cca.TiltDownButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X == 0 && shift.Y < 0, nil
	}}
	tiltUp = ptzControl{&cca.TiltUpButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X == 0 && shift.Y > 0, nil
	}}
	zoomIn = ptzControl{&cca.ZoomInButton, func(r, r2 *image.Rectangle) (bool, error) {
		return r.Size().X < r2.Size().X && r.Size().Y < r2.Size().Y, nil
	}}
	zoomOut = ptzControl{&cca.ZoomOutButton, func(r, r2 *image.Rectangle) (bool, error) {
		return r.Size().X > r2.Size().X && r.Size().Y > r2.Size().Y, nil
	}}
)

// testToggle tests toggling the control |ctrl|.
func (ctrl *ptzControl) testToggle(ctx context.Context, app *cca.App) error {
	pRect, err := findPattern(ctx, app)
	if err != nil {
		return errors.Wrapf(err, "failed to find pattern before clicking %v: %v", ctrl.ui.Name, err)
	}
	if err := app.ClickPTZButton(ctx, *ctrl.ui); err != nil {
		return errors.Wrapf(err, "failed to click: %v", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rect, err := findPattern(ctx, app)
		if err != nil {
			return errors.Wrapf(err, "failed to find pattern after clicking %v: %v", ctrl.ui.Name, err)
		}
		result, err := ctrl.testFunc(pRect, rect)
		if err != nil {
			return testing.PollBreak(err)
		}
		if result {
			return nil
		}
		return errors.Errorf("failed on testing UI with region before %v ; after %v", pRect, rect)
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to run %v test func: %v", ctrl.ui.Name, err)
	}
	return nil
}

func CCAUIPTZ(ctx context.Context, s *testing.State) {
	y4m, err := preparePattern()
	if err != nil {
		s.Fatal("Failed to prepare temp y4m: ", y4m)
	}
	defer os.Remove(y4m)

	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		"--use-fake-device-for-media-stream=fps=30",
		"--use-file-for-fake-video-capture="+y4m))
	if err != nil {
		s.Fatalf("Failed to start chrome with file source %v: %v", y4m, err)
	}
	defer cr.Close(ctx)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	if err := app.Click(ctx, cca.OpenPTZPanelButton); err != nil {
		s.Fatal("Failed to open ptz panel: ", err)
	}

	// Test move all controls. The controls need to be tested in order such
	// that |zoomIn| before all other controls(For all other controls will
	// be disabled in minimal zoom level as behavior of digital zoom
	// camera), |panLeft| before |panRight| (For the initial pan level is 0
	// with range [0, 15]) with initial mirror state, |tiltDown| before
	// |tiltUp| (For the initial tilt level is 0 with range[0, 8]).
	for _, control := range []ptzControl{
		zoomIn,
		panLeft,
		panRight,
		tiltDown,
		tiltUp,
		zoomOut,
	} {
		if err := control.testToggle(ctx, app); err != nil {
			s.Fatal("Failed: ", err)
		}
	}
}
