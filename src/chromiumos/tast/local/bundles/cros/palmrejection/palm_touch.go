// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package palmrejection contains local Tast tests that test palm rejection for different devices
package palmrejection

import (
	"context"
	"encoding/csv"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PalmTouch,
		Desc:         "Test palm rejection functionality",
		Contacts:     []string{"myy@chromium.org", "chromeos-touch-experience@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model("kohaku", "coachz")),
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name:              "kohaku",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Val: testParam{
				dataFiles: []string{"kohaku-palmedgeinmiddle.csv"},
			},
		}, {
			Name:              "coachz",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("coachz")),
			Val: testParam{
				dataFiles: []string{"coachz-palmedgeinmiddle.csv"},
			},
		}},
		Data: []string{"touch.html", "touch.mjs", "kohaku-palmedgeinmiddle.csv", "coachz-palmedgeinmiddle.csv"},
	})
}

type testParam struct {
	dataFiles []string
}

type evTestRecord struct {
	time        float64
	trackingID  int
	end         int
	x           float64
	y           float64
	radius      float64
	minorRadius float64
	pressure    float64
}

func readEvtestFile(s *testing.State, dataFile string) []evTestRecord {
	b, err := ioutil.ReadFile(s.DataPath(dataFile))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	}
	reader := csv.NewReader(strings.NewReader(string(b)))

	if _, err := reader.Read(); err != nil {
		s.Fatal("Failed to read header line: ", err)
	}

	var evtest []evTestRecord

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			s.Fatal("Failed to read line: ", err)
		}
		time, err := strconv.ParseFloat(line[0], 64)
		if err != nil {
			s.Fatal("Failed to parse time: ", err)
		}
		trackingID, err := strconv.Atoi(line[1])
		if err != nil {
			s.Fatal("Failed to parse trackingID: ", err)
		}
		end, err := strconv.Atoi(line[2])
		if err != nil {
			s.Fatal("Failed to parse end: ", err)
		}
		x, err := strconv.ParseFloat(line[3], 64)
		if err != nil {
			s.Fatal("Failed to parse x: ", err)
		}
		y, err := strconv.ParseFloat(line[4], 64)
		if err != nil {
			s.Fatal("Failed to parse y: ", err)
		}
		radius, err := strconv.ParseFloat(line[5], 64)
		if err != nil {
			s.Fatal("Failed to parse radius: ", err)
		}
		minorRadius, err := strconv.ParseFloat(line[6], 64)
		if err != nil {
			s.Fatal("Failed to parse minorRadius: ", err)
		}
		pressure, err := strconv.ParseFloat(line[7], 64)
		if err != nil {
			s.Fatal("Failed to parse pressure: ", err)
		}
		evtest = append(evtest, evTestRecord{
			time:        time,
			trackingID:  trackingID,
			end:         end,
			x:           x,
			y:           y,
			radius:      radius,
			minorRadius: minorRadius,
			pressure:    pressure,
		})
	}
	return evtest
}

func runEvtest(ctx context.Context, s *testing.State, tsw *input.TouchscreenEventWriter, evtest []evTestRecord) {
	sleep := func(t time.Duration) {
		if err := testing.Sleep(ctx, t); err != nil {
			s.Fatal("Timeout reached: ", err)
		}
	}
	trackingIDStwMap := make(map[int32]*input.SingleTouchEventWriter)
	lastTime := evtest[0].time
	for _, d := range evtest {
		sleep(time.Duration((d.time - lastTime)) * time.Second)
		lastTime = d.time

		_, ok := trackingIDStwMap[d.trackingID]
		if !ok {
			currentStw, err := tsw.NewSingleTouchWriter()
			if err != nil {
				s.Fatal("Could not create TouchEventWriter: ", err)
			}
			defer currentStw.Close()
			trackingIDStwMap[trackingID] = currentStw
		}
		currentStw := trackingIDStwMap[trackingID]
		currentStw.SetSize(ctx, int32(d.radius), int32(d.minorRadius))
		currentStw.SetPressure(int32(d.pressure))

		if err := currentStw.Move(input.TouchCoord(d.x), input.TouchCoord(d.y)); err != nil {
			s.Error("Failed to move: ", err)
		}

		if d.end == 1 {
			if err := currentStw.End(); err != nil {
				s.Error("Failed to finish the touch gesture: ", err)
			}
		}
	}
}

func PalmTouch(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Setup and start webserver.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Open the browser and navigate to the touch test page.
	url := server.URL + "/touch.html"

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.getElementById('preview')"); err != nil {
		s.Fatal("Timed out waiting for page load: ", err)
	}

	s.Log("Finding and opening touchscreen device")
	// It is possible to send raw events to the Touchscreen type. But it is recommended to
	// use the Touchscreen.TouchEventWriter struct since it already has functions to manipulate
	// Touch events.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()

	paramVal := s.Param().(testParam)
	for _, dataFile := range paramVal.dataFiles {
		evtest := readEvtestFile(s, dataFile)
		runEvtest(ctx, s, tsw, evtest)
		if err := conn.WaitForExpr(ctx, "document.getElementsByClassName('accept').length == 0"); err != nil {
			s.Fatal("Palm is not rejected: ", err)
		}
	}
}
