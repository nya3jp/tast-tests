// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
	packageCStateFile = "/sys/kernel/debug/pmc_core/package_cstate_show"
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
		Func:         TouchpadSuspendResume,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Touchpad: suspend-resume with operation for 10 cycles",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"canvas.html"},
		Timeout:      15 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.Touchpad(), hwdep.X86()),
		Fixture:      "chromeLoggedIn",
	})
}

func TouchpadSuspendResume(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open Test API connection: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get Internal display info: ", err)
	}

	cmd, stdout, err := touchPadScanner(ctx)
	if err != nil {
		s.Fatal("Failed to get touchpad scanner: ", err)
	}
	defer cmd.Wait()
	defer cmd.Kill()

	scannerTouchpad := bufio.NewScanner(stdout)

	if err := performEVTest(ctx, info, cr, scannerTouchpad, srv.URL); err != nil {
		s.Fatal("Failed to perform evtest: ", err)
	}

	cmdOutput := func(file string) string {
		out, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatalf("Failed to read %q file: %v", file, err)
		}
		return string(out)
	}

	slpPreString := cmdOutput(slpS0File)
	pkgOpSetOutput := cmdOutput(packageCStateFile)
	matchSetPre := c10PackageRe.FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]

	s.Log("Executing suspend_stress_test for 10 cycles")
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

	if err := performEVTest(ctx, info, cr, scannerTouchpad, srv.URL); err != nil {
		s.Fatal("Failed to perform evtest: ", err)
	}

	if err := assertSLPCounter(slpPreString); err != nil {
		s.Fatal("Failed to assert SLP counter: ", err)
	}

	if err := assertPackageCState(pkgOpSetPre); err != nil {
		s.Fatal("Failed to assert package C-State: ", err)
	}
}

// performEVTest launches the offline canvas, draw on the canvas using touchpad and
// monitors the touch events in evtest.
func performEVTest(ctx context.Context, info *display.Info, cr *chrome.Chrome, scanner *bufio.Scanner, testServerURL string) error {
	if err := launchCanvas(ctx, cr, testServerURL); err != nil {
		return errors.Wrap(err, "failed to launch canvas")
	}

	if err := drawOnCanvas(ctx, info); err != nil {
		return errors.Wrap(err, "failed to draw on canvas")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Monitor touchpad events using evtest.
	if err := evtestMonitor(timeoutCtx, scanner); err != nil {
		return errors.Wrap(err, "failed during the evtest for touchpad")
	}
	return nil
}

// launchCanvas loads the offline canvas.html in browser.
func launchCanvas(ctx context.Context, cr *chrome.Chrome, testServerURL string) error {
	conn, err := cr.NewConn(ctx, testServerURL+"/canvas.html")
	if err != nil {
		return errors.Wrap(err, "failed to load canvas.html")
	}
	if err := conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close connection to browser tab")
	}
	return nil
}

// assertSLPCounter asserts the SLP counter value post Resume with SLP counter
// value before Suspend.
func assertSLPCounter(slpPreString string) error {
	slpOpSetPost, err := ioutil.ReadFile(slpS0File)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", slpS0File)
	}
	slpPostString := string(slpOpSetPost)
	if slpPreString == slpPostString {
		return errors.Errorf("failed SLP counter value must be different than the value %q noted most recently %q", slpPreString, slpPostString)
	}
	if slpPostString == "0" {
		return errors.Errorf("failed SLP counter value must be non-zero, noted is: %q", slpOpSetPost)
	}
	return nil
}

// assertPackageCState asserts the Package C10 value post Resume with Package
// C10 value before Suspend.
func assertPackageCState(pkgOpSetPre string) error {
	pkgOpSetPostOutput, err := ioutil.ReadFile(packageCStateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q file", packageCStateFile)
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

// touchPadScanner returns the evtest scanner for the touch pad device.
func touchPadScanner(ctx context.Context) (*testexec.Cmd, io.Reader, error) {
	foundTS, devPath, err := input.FindPhysicalTrackpad(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to find device path for the touchpad")
	}
	if !foundTS {
		return nil, nil, errors.New("failed to find physical touchpad")
	}
	cmd := testexec.CommandContext(ctx, "evtest", devPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start scanner")
	}

	return cmd, stdout, nil
}

// evtestMonitor is used to check whether events sent to the devices are picked up by the evtest.
func evtestMonitor(ctx context.Context, scanner *bufio.Scanner) error {
	text := make(chan string)
	go func() {
		for scanner.Scan() {
			text <- scanner.Text()
		}
		close(text)
	}()
	for {
		select {
		case <-ctx.Done():
			return errors.New("failed to detect events within expected time")
		case out := <-text:
			if evtestRe.MatchString(out) {
				return nil
			}
		}
	}
}

// drawOnCanvas draws on the canvas using TouchEventWriter.
func drawOnCanvas(ctx context.Context, info *display.Info) error {
	tsw, err := input.Trackpad(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open trackpad device")
	}
	defer tsw.Close()

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

	const boxSize = 400 // box size in pixels.
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
