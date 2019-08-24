// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Downloads,
		Desc:         "Checks Downloads integration is working",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"capybara.jpg"},
		Pre:          arc.Booted(),
	})
}

func Downloads(ctx context.Context, s *testing.State) {
	const (
		filename    = "capybara.jpg"
		crosPath    = "/home/chronos/user/Downloads/" + filename
		androidPath = "/storage/emulated/0/Download/" + filename
	)

	a := s.PreValue().(arc.PreData).ARC

	expected, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatal("Could not read the test file: ", err)
	}

	// CrOS -> Android
	if err = ioutil.WriteFile(crosPath, expected, 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", crosPath, err)
	}
	actual, err := a.ReadFile(ctx, androidPath)
	if err != nil {
		s.Error("CrOS -> Android failed: ", err)
	} else if !bytes.Equal(actual, expected) {
		s.Error("CrOS -> Android failed: content mismatch")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}

	// Android -> CrOS
	if err = a.WriteFile(ctx, androidPath, expected); err != nil {
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
