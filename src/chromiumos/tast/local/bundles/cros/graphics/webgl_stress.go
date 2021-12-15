// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type webglRun struct {
	minute time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebGLStress,
		Desc:         "Verify WebGl Aquarium multiple Windows stress long duration",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Data:         []string{"webgl_aquarium_static.tar.bz2"},
		Params: []testing.Param{{
			Name:      "smoke",
			ExtraAttr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
			Val:       webglRun{minute: 1},
		}, {
			Name:    "bronze",
			Val:     webglRun{minute: 6 * 60}, // stress test runs for 6 hours.
			Timeout: 6*time.Hour + 10*time.Minute,
		}, {
			Name:    "silver",
			Val:     webglRun{minute: 9 * 60}, // stress test runs for 9 hours.
			Timeout: 9*time.Hour + 10*time.Minute,
		}, {
			Name:    "gold",
			Val:     webglRun{minute: 12 * 60}, // stress test runs for 12 hours.
			Timeout: 12*time.Hour + 10*time.Minute,
		},
		}})
}

// WebGLStress opens webgl page in multiple windows, run for provided duration
// and calculate average fps value for each webgl window.
func WebGLStress(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	testRun := s.Param().(webglRun)
	const totalWindows = 10

	var webglStatic = s.DataPath("webgl_aquarium_static.tar.bz2")
	dataPath, filename := filepath.Split(webglStatic)
	if err := testexec.CommandContext(ctx, "sudo", "tar", "-xvf", webglStatic, "-C", dataPath).Run(); err != nil {
		s.Fatal("Failed to extract tar file: ", err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(filepath.Join(dataPath, strings.Split(filename, ".")[0]))))
	defer server.Close()

	shortCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var ws []*ash.Window
	conns := make(map[int]map[string]interface{})
	for i := 1; i <= totalWindows; i++ {
		conn, err := cr.NewConn(ctx, server.URL+"/aquarium.html", browser.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open webgl test page: ", err)
		}
		defer conn.Close()

		if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
			s.Fatal("Failed to load webgl page: ", err)
		}

		// Set fish count to 1000.
		if err := conn.Eval(ctx, fmt.Sprintf(
			`(function() {
				  setSetting(document.getElementById("%s"), %d);
					g_crosFpsCounter.reset();
				})()`,
			"setSetting6", 6), nil); err != nil {
			s.Fatal("Failed to set fish count: ", err)
		}

		// Get all opened browser windows.
		ws, err = ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get webgl windows: ", err)
		}
		conns[i] = map[string]interface{}{"wObj": ws[0], "webglConn": conn}
	}

	if wsCount := len(ws); wsCount != totalWindows {
		s.Fatalf("Failed to run all webgl windows: got %d, want %d", wsCount, totalWindows)
	}

	if err := verifyHardwareAccelWebGL(shortCtx, cr); err != nil {
		s.Fatal("Failed to verify webgl hardware accelerated: ", err)
	}

	fpsMap := make(map[int]int)
	fpsMapKeys := make([]int, 0, totalWindows)
	endTime := time.Now().Add(testRun.minute * time.Minute)

	fps := 30
	iter := 0

	for {
		timeNow := time.Now()
		if timeNow.After(endTime) {
			break
		}

		s.Logf("Webgl is running in %d windows, test remaining time: %v", totalWindows, endTime.Sub(timeNow))
		for pos, wConn := range conns {
			w := wConn["wObj"].(*ash.Window)
			webglConn := wConn["webglConn"].(*chrome.Conn)
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				s.Fatalf("Failed to activate the window %d: %v", pos, err)
			}
			val := 0
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var fpsVal string
				if err := webglConn.Eval(ctx, "document.getElementById('fps').innerText", &fpsVal); err != nil {
					s.Fatal("Failed to get fps value for webgl window: ", err)
				}
				fpsNum, err := strconv.Atoi(fpsVal)
				if err != nil {
					s.Fatal("Failed to convert fps value to int: ", err)
				}
				if fpsNum < fps {
					return errors.Errorf("webgl fps value; got %d, want greater than %d", val, fps)
				}
				val = fpsNum
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
				s.Fatal("Failed to verify fps: ", err)
			}
			fpsMap[pos] += val
		}

		iter++
		if iter == 1 {
			for key := range fpsMap {
				fpsMapKeys = append(fpsMapKeys, key)
			}
			sort.Ints(fpsMapKeys)
		}
		for _, key := range fpsMapKeys {
			s.Logf("Average fps value for webgl window %d: %d", key, fpsMap[key]/iter)
		}
		// After every 5 minutes, calculating average FPS value.
		testing.Sleep(ctx, 5*time.Minute)
	}
}

// verifyHardwareAccelWebGL lanches chrome://gpu page and checks webgl and webgl2 hardware accelerated
// in hardware feature list.
func verifyHardwareAccelWebGL(ctx context.Context, cr *chrome.Chrome) error {
	gpuConn, err := cr.NewConn(ctx, "chrome://gpu", browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open chrome gpu url")
	}
	defer gpuConn.Close()

	var gpuInfo string
	if err := gpuConn.Eval(ctx, "document.getElementsByClassName('feature-status-list')[0].innerText", &gpuInfo); err != nil {
		return errors.Wrap(err, "failed to get graphics feature status")
	}
	webglRe := regexp.MustCompile(`(?i)WebGL: Hardware accelerated`)
	webgl2Re := regexp.MustCompile(`(?i)WebGL2: Hardware accelerated`)
	if !(webglRe.MatchString(gpuInfo) && webgl2Re.MatchString(gpuInfo)) {
		return errors.New("WebGL or WebGL2 hardware accelerated is not found")
	}
	return nil
}
