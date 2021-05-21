// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strconv"

	"chromiumos/tast/autocaps"
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
		Attr: []string{"group:mainline", "group:camera-libcamera"},
	})
}

func Characteristics(ctx context.Context, s *testing.State) {
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	if err != nil {
		s.Fatal("Failed to read static capabilities: ", err)
	}
	hasBuiltinUsbCamera := false
	if c, ok := staticCaps["builtin_usb_camera"]; ok && c == autocaps.Yes {
		hasBuiltinUsbCamera = true
	}

	const gtestExecutableName = "camera_characteristics_test"
	t := gtest.New(gtestExecutableName,
		gtest.ExtraArgs("--skip_if_no_config="+strconv.FormatBool(!hasBuiltinUsbCamera)),
		gtest.Logfile(filepath.Join(s.OutDir(), gtestExecutableName+".log")))
	args, err := t.Args()
	if err != nil {
		s.Fatal("Failed to get GTest execution args: ", err)
	}
	s.Log("Running ", shutil.EscapeSlice(args))
	if _, err := t.Run(ctx); err != nil {
		s.Fatalf("Failed to run %v: %v", gtestExecutableName, err)
	}
}
