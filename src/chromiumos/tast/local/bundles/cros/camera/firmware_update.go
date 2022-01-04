// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

const defaultJSONConfigPath = "/etc/camera/fw_test_config.json"
const gtestExecutableName = "camera_dfu_test"

func init() {
	testing.AddTest(&testing.Test{
		Func: FirmwareUpdate,
		Desc: "Exercises firmware update on USB camera",
		Contacts: []string{
			"kamesan@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Vars: []string{"config"},
	})
}

func FirmwareUpdate(ctx context.Context, s *testing.State) {
	jsonConfigPath, hasJSONConfigPath := s.Var("config")
	if !hasJSONConfigPath {
		jsonConfigPath = defaultJSONConfigPath
	}

	bytes, err := ioutil.ReadFile(jsonConfigPath)
	if err != nil {
		s.Fatalf("Failed to read %v: %v", jsonConfigPath, err)
	}

	var testCfg map[string]string
	if err := json.Unmarshal(bytes, &testCfg); err != nil {
		s.Fatalf("Failed to parse %v in JSON: %v", jsonConfigPath, err)
	}

	var args []string
	for k, v := range testCfg {
		args = append(args, "--"+k+"="+v)
	}
	// Enable verbose logs.
	args = append(args, "--v=1")

	t := gtest.New(gtestExecutableName,
		gtest.ExtraArgs(args...),
		gtest.Logfile(filepath.Join(s.OutDir(), gtestExecutableName+".log")))
	if _, err := t.Run(ctx); err != nil {
		s.Fatalf("Failed to run %v: %v", gtestExecutableName, err)
	}
}
