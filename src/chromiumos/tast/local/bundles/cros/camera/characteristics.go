// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Characteristics,
		Desc: "Verifies the format of camera characteristics file",
		Contacts: []string{
			"kamesan@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Characteristics(ctx context.Context, s *testing.State) {
	const gtestExecutableName = "camera_characteristics_test"
	t := gtest.New(gtestExecutableName,
		gtest.Logfile(filepath.Join(s.OutDir(), gtestExecutableName+".log")))
	if args, err := t.Args(); err == nil {
		s.Log("Running ", shutil.EscapeSlice(args))
	}
	if _, err := t.Run(ctx); err != nil {
		s.Fatalf("Failed to run %v: %v", gtestExecutableName, err)
	}
}
