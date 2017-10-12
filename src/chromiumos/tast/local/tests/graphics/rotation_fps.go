// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"fmt"
	"strings"

	"chromiumos/tast/common/testing"
	"chromiumos/tast/local/chrome"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RotationFPS,
		Desc: "Measures the frame rate while rotating the screen",
		Attr: []string{"chrome"},
	})
}

func RotationFPS(s *testing.State) {
	const (
		rotationDegrees = 180
		histogramsURL   = "chrome://histograms"
		histogramName   = "Ash.Rotation.AnimationSmoothness"
	)

	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	ext, err := cr.TestExtConn(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to test extension: ", err)
	}

	infos := make([]interface{}, 0)
	err = ext.EvalPromise(s.Context(),
		`new Promise(function(resolve, reject) {
			chrome.system.display.getInfo(function(info) { resolve(info); });
		})`, &infos)
	if err != nil {
		s.Fatal("Failed to get display infos: ", err)
	}
	if len(infos) == 0 {
		s.Fatal("No displays found")
	}
	s.Logf("Got %d display(s)", len(infos))

	var id string
	if info, ok := infos[0].(map[string]interface{}); ok {
		id, _ = info["id"].(string)
	}
	if id == "" {
		s.Fatal("Failed to get display ID")
	}

	s.Log("Rotating display ", id)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.system.display.setDisplayProperties(
				%q, {'rotation': %d}, function() {
					resolve(chrome.runtime.lastError == null);
				});
		})`, id, rotationDegrees)
	success := false
	if err = ext.EvalPromise(s.Context(), expr, &success); err != nil {
		s.Fatal("Failed to rotate display: ", err)
	}

	s.Log("Navigating to ", histogramsURL)
	conn, err := cr.NewConn(s.Context(), "")
	if err != nil {
		s.Fatal("Failed to open tab")
	}
	if err = conn.Navigate(s.Context(), histogramsURL); err != nil {
		s.Fatal("Failed to load histograms page: ", err)
	}

	var data string
	if err = conn.Eval(s.Context(), "document.documentElement.innerText", &data); err != nil {
		s.Fatal("Failed to read histograms data: ", err)
	}
	var mean string
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.Contains(line, histogramName) {
			// TODO: Grab the mean from the line and report it.
		}
	}
}
