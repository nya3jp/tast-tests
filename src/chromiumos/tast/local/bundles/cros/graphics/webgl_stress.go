// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type webglRun struct {
	hour int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebglStress,
		Desc:         "Verifies WebGl Aquarium multiple Windows stress long duration",
		Contacts:     []string{"intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:graphics"},
		Params: []testing.Param{{
			Name:    "bronze",
			Val:     webglRun{hour: 6},
			Timeout: 370 * time.Minute,
			Fixture: "chromeLoggedIn",
		},
			{
				Name:    "silver",
				Val:     webglRun{hour: 9},
				Timeout: 550 * time.Minute,
				Fixture: "chromeLoggedIn",
			}, {
				Name:    "gold",
				Val:     webglRun{hour: 12},
				Timeout: 730 * time.Minute,
				Fixture: "chromeLoggedIn",
			},
		}})
}

// WebglStress opens webgl page in multiple windows, run for long duration
// and calculate average fps value for each webgl window.
func WebglStress(ctx context.Context, s *testing.State) {
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

	var webglURL = "https://webglsamples.org/aquarium/aquarium.html?numFish=5000"
	const totalWindows = 10

	conns := make(map[*ash.Window]*chrome.Conn)
	defer func() {
		for w, c := range conns {
			if err := c.Close(); err != nil {
				s.Logf("Failed to close the %d-th connection: %v", w.ID, err)
			}
		}
	}()

	shortCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var ws []*ash.Window
	for i := 1; i <= totalWindows; i++ {
		conn, err := cr.NewConn(shortCtx, webglURL, browser.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open webgl url: ", err)
		}
		// Wait until page has desired title to avoid race conditions.
		if err := conn.WaitForExpr(ctx, `document.title === "WebGL Aquarium"`); err != nil {
			s.Fatal("Failed to load webgl page: ", err)
		}

		// Get all opened browser windows.
		ws, err = ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get webgl windows: ", err)
		}
		conns[ws[0]] = conn
	}

	if wsCount := len(ws); wsCount != totalWindows {
		s.Fatalf("Expected %d webgl windows; found %d", totalWindows, wsCount)
	}

	if err := chromeGpu(shortCtx, cr); err != nil {
		s.Fatal("Failed to verify webgl hardware accelerated: ", err)
	}

	fpsMap := make(map[int]int)
	startTime := time.Now().Unix()
	endTime := float64(testRun.hour * 60 * 60)
	iter := 0
	index := 1

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if index == totalWindows {
			index = 1
		}
		for w, webglConn := range conns {
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				return testing.PollBreak(errors.Wrapf(err, "failed to activate the window %d", w.ID))
			}
			// Use 0.5 second because this is roughly the amount of time it takes
			// to get proper fps value after window get activated.
			testing.Sleep(ctx, 500*time.Millisecond)
			var fps string
			if err := webglConn.Eval(ctx, "document.getElementById('fps').innerText", &fps); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get fps value for webgl window"))
			}
			fpsVal, err := strconv.Atoi(fps)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to convert fps value to int"))
			}
			fpsMap[index] += fpsVal
			if index != totalWindows {
				index++
			}
		}
		iter++
		timeNow := float64(time.Now().Unix() - startTime)
		if timeNow < endTime {
			s.Logf("Webgl is running in %d windows, test remaining time: %f/%f sec", totalWindows, timeNow, endTime)
			return errors.New("webgl windows are running")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Duration(testRun.hour*60+25) * time.Minute, Interval: 5 * time.Minute}); err != nil {
		s.Fatal("Failed to run webgl in multiple windows: ", err)
	}

	Keys := make([]int, len(fpsMap))
	i := 0
	for key := range fpsMap {
		Keys[i] = key
		i++
	}

	// Sort keys in ascending order.
	sort.Ints(Keys)

	for _, key := range Keys {
		s.Logf("Average fps value for webgl window %d: %d", key, fpsMap[key]/iter)
	}
}

// chromeGpu lanches chrome://gpu page and checks webgl and webgl2 hardware accelerated
// in hardware feature list.
func chromeGpu(ctx context.Context, cr *chrome.Chrome) error {
	gpuConn, err := cr.NewConn(ctx, "chrome://gpu")
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
