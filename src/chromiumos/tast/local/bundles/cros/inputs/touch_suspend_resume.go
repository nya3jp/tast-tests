// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"bufio"
	"context"
	"io/ioutil"
	"math"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	slpS0File         = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
	packageCstateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
)

var (
	c10PackageRe       = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	suspendFailureRe   = regexp.MustCompile("Suspend failures: 0")
	firmwareLogErrorRe = regexp.MustCompile("Firmware log errors: 0")
	s0ixErrorRe        = regexp.MustCompile("s0ix errors: 0")
	evtestRe           = regexp.MustCompile(`Event.*time.*code\s(\d*)\s\(` + `ABS_MT_POSITION_X` + `\)`)
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TouchSuspendResume,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Touchscreen: suspend-resume with operation for 10 cycles",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.X86()),
		Fixture:      "chromeLoggedIn",
	})
}

func TouchSuspendResume(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open Test API connection: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get Internal display info: ", err)
	}

	scannTouchscreen, err := deviceScanner(ctx)
	if err != nil {
		s.Fatal("Failed to get touchscreen scanner: ", err)
	}

	if err := launchCanvas(ctx, cr); err != nil {
		s.Fatal("Failed to launch canvas: ", err)
	}

	if err := drawOnCanvas(ctx, info); err != nil {
		s.Fatal("Failed to draw on canvas: ", err)
	}
	// Monitor touchscreen events using evtest.
	if err := evtestMonitor(scannTouchscreen); err != nil {
		s.Fatal("Failed during the evtest for touchscreen: ", err)
	}

	cmdOutput := func(file string) string {
		out, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatalf("Failed to read %q file: %v", file, err)
		}
		return string(out)
	}

	slpOpSetPre := cmdOutput(slpS0File)
	pkgOpSetOutput := cmdOutput(packageCstateFile)
	matchSetPre := c10PackageRe.FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]

	testing.ContextLog(ctx, "Executing suspend_stress_test for 10 cycles")
	stressOut, err := testexec.CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}
	suspendErrors := []*regexp.Regexp{suspendFailureRe, firmwareLogErrorRe, s0ixErrorRe}
	for _, errmsg := range suspendErrors {
		if !(errmsg.MatchString(string(stressOut))) {
			s.Fatalf("Failed expected %q, but failures are non-zero", errmsg)
		}
	}

	// re-establishing chrome connection to DUT.
	if err := cr.Reconnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to the Chrome session: ", err)
	}

	if err := launchCanvas(ctx, cr); err != nil {
		s.Fatal("Failed to launch canvas: ", err)
	}

	if err := drawOnCanvas(ctx, info); err != nil {
		s.Fatal("Failed to draw on canvas: ", err)
	}
	// Monitor touchscreen events using evtest.
	if err := evtestMonitor(scannTouchscreen); err != nil {
		s.Fatal("Failed during the evtest for touchscreen: ", err)
	}

	if err := assertSLPCounter(slpOpSetPre); err != nil {
		s.Fatal("Failed to assert SLP counter: ", err)
	}

	if err := assertPackageCState(pkgOpSetPre); err != nil {
		s.Fatal("Failed to assert package C-State: ", err)
	}
}

// launchCanvas launches the paint tool to draw using touch.
func launchCanvas(ctx context.Context, cr *chrome.Chrome) error {
	conn, err := cr.NewConn(ctx, "http://kleki.com")
	if err != nil {
		return errors.Wrap(err, "failed to open link")
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('canvas').length > 0"); err != nil {
		return errors.Wrap(err, "failed to wait for the page to load")
	}
	return nil
}

// assertSLPCounter asserts the SLP counter value post Resume with SLP counter
// value before Suspend.
func assertSLPCounter(slpOpSetPre string) error {
	slpOpSetPost, err := ioutil.ReadFile(slpS0File)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", slpS0File)
	}
	if slpOpSetPre == string(slpOpSetPost) {
		return errors.Errorf("failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
	}
	if string(slpOpSetPost) == "0" {
		return errors.Errorf("failed SLP counter value must be non-zero, noted is: %q", slpOpSetPost)
	}
	return nil
}

// assertPackageCState asserts the Package C10 value post Resume with Package
// C10 value before Suspend.
func assertPackageCState(pkgOpSetPre string) error {
	pkgOpSetPostOutput, err := ioutil.ReadFile(packageCstateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", packageCstateFile)
	}
	matchSetPost := c10PackageRe.FindStringSubmatch(string(pkgOpSetPostOutput))
	if matchSetPost == nil {
		return errors.Errorf("failed to match post PkgCstate value: %q", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		return errors.Errorf("failed Package C10 value %q must be different than value %q noted most recently", pkgOpSetPre, pkgOpSetPost)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		return errors.Errorf("failed Package C10 = want non-zero, got %s", pkgOpSetPost)
	}
	return nil
}

// deviceScanner returns the evtest scanner for the touch screen device.
func deviceScanner(ctx context.Context) (*bufio.Scanner, error) {
	foundTS, devPath, err := input.FindPhysicalTouchscreen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find device path for the touch screen")
	}
	if !foundTS {
		return nil, errors.New("failed to find physical touch screen")
	}
	cmd := testexec.CommandContext(ctx, "evtest", devPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start scanner")
	}

	scanner := bufio.NewScanner(stdout)
	return scanner, nil
}

// evtestMonitor is used to check whether events sent to the devices are picked up by the evtest.
func evtestMonitor(scanner *bufio.Scanner) error {
	const scanTimeout = 5 * time.Second
	text := make(chan string)
	go func() {
		for scanner.Scan() {
			text <- scanner.Text()
		}
		close(text)
	}()
	for {
		select {
		case <-time.After(scanTimeout):
			return errors.New("failed to detect events within expected time")
		case out := <-text:
			if match := evtestRe.FindStringSubmatch(out); match != nil {
				return nil
			}
		}
	}
}

// drawOnCanvas draws on the canvas using TouchEventWriter.
func drawOnCanvas(ctx context.Context, info *display.Info) error {
	// It is possible to send raw events to the Touchscreen type. But it is recommended to
	// use the Touchscreen.TouchEventWriter struct since it already has functions to manipulate
	// Touch events.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	touchWidth := tsw.Width()
	touchHeight := tsw.Height()

	// Display bounds.
	displayWidth := float64(info.Bounds.Width)
	displayHeight := float64(info.Bounds.Height)

	pixelToTouchFactorX := float64(touchWidth) / displayWidth
	pixelToTouchFactorY := float64(touchHeight) / displayHeight

	centerX := displayWidth * pixelToTouchFactorX / 2
	centerY := displayHeight * pixelToTouchFactorY / 2

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to get a new TouchEventWriter")
	}
	defer stw.Close()

	// Draw a dotted line:
	// SingleTouchEventWriter is being reused for the 15 dots. The event is "ended" after each touch.
	// "End" is equivalent as lifting the finger from the touchscreen.
	// Thus generating a "dotted" line, instead of continuos one.
	for i := 0; i < 15; i++ {
		// Values must be in "touchscreen coordinates", not pixel coordinates.
		stw.Move(input.TouchCoord(centerX+float64(i)*50.0), input.TouchCoord(centerY+float64(i)*50.0))
		stw.End()
		testing.Sleep(ctx, 100*time.Millisecond)
	}

	// Draw a circle:
	// Draws a circle with 120 touch events. The touch event is moved to
	// 120 different locations generating a continuous circle.
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create TouchEventWriter")
	}
	defer stw.Close()

	const (
		radius   = 200 // circle radius in pixels.
		segments = 120 // segments used for the circle.
	)
	for i := 0; i < segments; i++ {
		rads := 2.0*math.Pi*(float64(i)/segments) + math.Pi
		x := radius * pixelToTouchFactorX * math.Cos(rads)
		y := radius * pixelToTouchFactorY * math.Sin(rads)
		if err := stw.Move(input.TouchCoord(centerX+x), input.TouchCoord(centerY+y)); err != nil {
			return errors.Wrap(err, "failed to move the touch event")
		}
		testing.Sleep(ctx, 15*time.Millisecond)
	}
	// And finally "end" (lift the finger) the line.
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the touch event")
	}

	// Swipe test:
	// Draw a box around the circle using 4 swipes.
	const boxSize = radius * 2 // box size in pixels
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create TouchEventWriter")
	}
	defer stw.Close()
	for _, d := range []struct {
		x0, y0, x1, y1 float64
	}{
		{-1, 1, -1, -1}, // swipe up form bottom-left.
		{-1, -1, 1, -1}, // swipe right from top-left.
		{1, -1, 1, 1},   // swipe down from top-right.
		{1, 1, -1, 1},   // swipe left from bottom-right.
	} {
		x0 := input.TouchCoord(centerX + boxSize/2*d.x0*pixelToTouchFactorX)
		y0 := input.TouchCoord(centerY + boxSize/2*d.y0*pixelToTouchFactorY)
		x1 := input.TouchCoord(centerX + boxSize/2*d.x1*pixelToTouchFactorX)
		y1 := input.TouchCoord(centerY + boxSize/2*d.y1*pixelToTouchFactorY)

		if err := stw.Swipe(ctx, x0, y0, x1, y1, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to run Swipe")
		}
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}
	return nil
}
