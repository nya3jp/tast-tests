// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Downloads,
		Desc:         "Checks Downloads integration is working",
		Attr:         []string{"bvt"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"capybara.jpg"},
	})
}

func Downloads(s *testing.State) {
	const (
		filename    = "capybara.jpg"
		crosPath    = "/home/chronos/user/Downloads/" + filename
		androidPath = "/storage/emulated/0/Download/" + filename
	)

	cr, err := chrome.New(s.Context(), chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	expected, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatal("Could not read the test file: ", err)
	}

	// CrOS -> Android
	if err = ioutil.WriteFile(crosPath, expected, 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", crosPath, err)
	}
	actual, err := arc.ReadFile(s.Context(), androidPath)
	if err != nil {
		s.Error("CrOS -> Android failed: ", err)
	} else if !bytes.Equal(actual, expected) {
		s.Error("CrOS -> Android failed: content mismatch")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}

	// Android -> CrOS
	if err = arc.WriteFile(s.Context(), androidPath, expected); err != nil {
		s.Fatalf("Could not write to %s: %v", androidPath, err)
	}
	actual, err = ioutil.ReadFile(crosPath)
	if err != nil {
		s.Error("Android -> CrOS failed: ", err)
	} else if !bytes.Equal(actual, expected) {
		s.Error("Android -> CrOS failed: content mismatch")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}
}
