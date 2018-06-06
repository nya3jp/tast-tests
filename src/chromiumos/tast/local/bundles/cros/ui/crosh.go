// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Crosh,
		Desc:         "Launches the crosh terminal",
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Crosh(s *testing.State) {
	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	// Interestingly, the cdp package blocks CreateURL requests with non-http/https schemes
	// like chrome-extension (presumably because Chrome would reject them later), but loading
	// a blank page and then navigating to a chrome-extension URL seems to work.
	conn, err := cr.NewConn(s.Context(), "")
	if err != nil {
		s.Fatal("Failed to create browser: ", err)
	}
	// crosh seems to use "chrome-extension://pnhechapfaindjhompbnflcldabbghjo/html/crosh.html"
	// on a stable-channel device, but this is what I see it use on a ToT dev device.
	const url = "chrome-extension://nkoccljplnhpfnfiajclkommnmllphnl/html/crosh.html"
	if err = conn.Navigate(s.Context(), url); err != nil {
		s.Fatalf("Failed to navigate to %v: %v", url, err)
	}

	s.Log("Waiting for term_ object")
	if err = conn.WaitForExpr(s.Context(), "!!term_"); err != nil {
		s.Fatal("Didn't find term_ object: ", err)
	}
	size := make(map[string]int)
	if err = conn.Eval(s.Context(), "term_.screenSize", &size); err != nil {
		s.Fatal(err)
	}
	s.Logf("Terminal has dimensions %dx%d", size["width"], size["height"])
}
