// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Characteristics,
		Desc: "Verifies the format of camera characteristics file for USB cameras",
		Contacts: []string{
			"kamesan@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
	})
}

func Characteristics(ctx context.Context, s *testing.State) {
	t := gtest.New("media_v4l2_test",
		gtest.Filter("ConfigTest.CharacteristicsFile"),
		gtest.Logfile(filepath.Join(s.OutDir(), "media_v4l2_test.log")))
	if args, err := t.Args(); err == nil {
		s.Log("Running ", shutil.EscapeSlice(args))
	}
	if _, err := t.Run(ctx); err != nil {
		s.Fatal("Failed to run media_v4l2_test: ", err)
	}
}
