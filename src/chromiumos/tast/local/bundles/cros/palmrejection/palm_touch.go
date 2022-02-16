// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package palmrejection contains tests that test palm rejection ability for different devices
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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PalmTouch,
		Desc:         "Test palm rejection functionality",
		Contacts:     []string{"myy@chromium.org", "chromeos-touch-ml@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name:              "kohaku",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraData:         []string{"palm_touch_kohaku_palmedgeinmiddle.csv"},
			Val: testParam{
				dataFiles: []string{"kohaku_palmedgeinmiddle.csv"},
			},
		}, {
			Name:              "coachz",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("coachz")),
			ExtraData:         []string{"palm_touch_coachz_palmedgeinmiddle.csv"},
			Val: testParam{
				dataFiles: []string{"coachz_palmedgeinmiddle.csv"},
			},
		}},
		Data: []string{"palm_touch_index.html", "palm_touch_script.mjs"},
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

func readEvtestFile(dataFile string) ([]evTestRecord, error) {
	var err error
	b, err := ioutil.ReadFile(dataFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read internal data file")
	}
	reader := csv.NewReader(strings.NewReader(string(b)))

	if _, err := reader.Read(); err != nil {
		return nil, errors.Wrap(err, "failed to read header line")
	}

	var evtest []evTestRecord

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to read line")
		}
		time, err := strconv.ParseFloat(line[0], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse time")
		}
		trackingID, err := strconv.Atoi(line[1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse trackingID")
		}
		end, err := strconv.Atoi(line[2])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse end")
		}
		x, err := strconv.ParseFloat(line[3], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse x")
		}
		y, err := strconv.ParseFloat(line[4], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse y")
		}
		radius, err := strconv.ParseFloat(line[5], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse radius")
		}
		minorRadius, err := strconv.ParseFloat(line[6], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse minorRadius")
		}
		pressure, err := strconv.ParseFloat(line[7], 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse pressure")
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
	return evtest, nil
}

func runEvtest(ctx context.Context, s *testing.State, tsw *input.TouchscreenEventWriter, evtest []evTestRecord) {
	sleep := func(t time.Duration) {
		if err := testing.Sleep(ctx, t); err != nil {
			s.Fatal("Timeout reached: ", err)
		}
	}
	trackingIDStwMap := make(map[int]*input.SingleTouchEventWriter)
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
			trackingIDStwMap[d.trackingID] = currentStw
		}
		currentStw := trackingIDStwMap[d.trackingID]
		currentStw.SetSize(ctx, int32(d.radius), int32(d.minorRadius))
		currentStw.SetPressure(int32(d.pressure))

		if err := currentStw.Move(input.TouchCoord(d.x), input.TouchCoord(d.y)); err != nil {
			s.Fatal("Failed to move: ", err)
		}

		if d.end == 1 {
			if err := currentStw.End(); err != nil {
				s.Fatal("Failed to finish the touch gesture: ", err)
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
	url := server.URL + "/palm_touch_index.html"

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		s.Fatal("Failed to wait for page to finish loading: ", err)
	}

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
		s.Run(ctx, dataFile, func(ctx context.Context, s *testing.State) {
			evtest, err := readEvtestFile(s.DataPath("palm_touch_" + dataFile))
			if err != nil {
				s.Fatal("Failed to read evtest file: ", err)
			}
			runEvtest(ctx, s, tsw, evtest)
			if err := conn.WaitForExpr(ctx, "document.getElementsByClassName('accept').length == 0"); err != nil {
				s.Fatal("Palm is not rejected: ", err)
			}
		})

	}
}
