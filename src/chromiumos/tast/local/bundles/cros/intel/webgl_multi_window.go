// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

var (
	webGLPattern  = regexp.MustCompile(`(?i)WebGL: Hardware accelerated`)
	webGL2Pattern = regexp.MustCompile(`(?i)WebGL2: Hardware accelerated`)
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebGLMultiWindow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "WebGL Aquarium on multi windows with 5000 fishes",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name:    "quick",
			Val:     2,
			Timeout: 5 * time.Minute,
		}, {
			Name:    "bronze",
			Val:     10,
			Timeout: 15 * time.Minute,
		}, {
			Name:    "silver",
			Val:     15,
			Timeout: 20 * time.Minute,
		}, {
			Name:    "gold",
			Val:     20,
			Timeout: 25 * time.Minute,
		}}})
}

func WebGLMultiWindow(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	t := s.Param().(int)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := verifyGPUAccel(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to verify WebGL & WebGL2 in chrome://gpu: ", err)
	}

	output, err := os.Create(filepath.Join(s.OutDir(), "intel_gpu.log"))
	if err != nil {
		s.Fatal("Failed to create intel_gpu log file: ", err)
	}
	defer output.Close()

	cmd := testexec.CommandContext(ctx, "intel_gpu_top", "-l")
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start command: ", err)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	conns, err := launchWebGL(ctx, tconn, cr, 3)
	if err != nil {
		s.Fatal("Failed to launch webGL url: ", err)
	}

	totalFPS := 0
	totalIterations := 0
	endTime := time.Now().Add(time.Duration(t) * time.Minute)
	s.Logf("Verifying WebGL for %d Minutes", t)
	for time.Now().Unix() < endTime.Unix() {
		for _, conn := range conns {
			defer conn.CloseTarget(cleanupCtx)
			var fps string
			fpsJS := `document.getElementById("fps").textContent`
			if err := conn.Eval(ctx, fpsJS, &fps); err != nil {
				s.Fatal("Failed to get FPS Count: ", err)
			}
			s.Logf("Current FPS Count: %s", fps)
			intFPS, err := strconv.Atoi(fps)
			if err != nil {
				s.Fatal("Failed to convert to int: ", err)
			}
			totalFPS += intFPS
			totalIterations++
			// sleeping for 30s to get average FPS.
			if err := testing.Sleep(ctx, 30*time.Second); err != nil {
				s.Fatal("Failed to sleep for 30 seconds: ", err)
			}
		}
	}
	averageFPS := totalFPS / totalIterations
	s.Logf("Average FPS Count: %d", averageFPS)
	if averageFPS < 30 {
		s.Errorf("Failed to maintain FPS more than 30: got %d", averageFPS)
	}

}

// launchWebGL launches url in multiple windows and returns the connections array.
func launchWebGL(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, numOfwindows int) ([]*chrome.Conn, error) {
	const webGLURL = "https://storage.googleapis.com/chromiumos-test-assets-public/power_ThermalLoad/aquarium/aquarium.html?numFish=5000"
	conns := make([]*chrome.Conn, 0, numOfwindows)
	for i := 0; i < numOfwindows; i++ {
		conn, err := cr.NewConn(ctx, webGLURL, browser.WithNewWindow())
		if err != nil {
			return nil, errors.Wrap(err, "failed to open webGL in new window")
		}
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain the window list")
		}

		browserWinID := ws[0].ID
		if _, err := ash.SetWindowState(ctx, tconn, browserWinID, ash.WMEventNormal, true /* waitForStateChange*/); err != nil {
			return nil, errors.Wrap(err, "failed to set the window state to normal")
		}
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == browserWinID && w.State == ash.WindowStateNormal && !w.IsAnimating
		}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "failed to wait for window to become normal")
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

// verifyGPUAccel verifies if webGL & webGL 2 is hardware accelerated or not.
func verifyGPUAccel(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	gpuConn, err := cr.NewConn(ctx, "chrome://gpu")
	if err != nil {
		return errors.Wrap(err, "failed to create a chrome://gpu connection")
	}
	defer gpuConn.Close()

	// copyButton is the finder for the Copy Report to Clipboard button.
	var copyButton = nodewith.Name("Copy Report to Clipboard").Role(role.Button)
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(copyButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click button")
	}

	gpuInfo, err := getClipboardText(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get clipboard data")
	}

	if !(webGLPattern.MatchString(gpuInfo) && webGL2Pattern.MatchString(gpuInfo)) {
		return errors.Wrap(err, "failed to find WebGL or WebGL2 hardware accelerated in GPU page")
	}
	return nil
}

// getClipboardText gets the clipboard text data.
func getClipboardText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var clipData string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return testing.PollBreak(err)
		}
		if clipData == "" {
			return errors.New("no clipboard data")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return "", errors.Wrap(err, "failed to get clipboard data")
	}
	return clipData, nil
}
