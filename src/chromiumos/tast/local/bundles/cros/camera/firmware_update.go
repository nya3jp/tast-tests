// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

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
	const (
		defaultJSONConfigPath = "/etc/camera/fw_test_config.json"
		gtestExecutableName   = "camera_dfu_test"
	)

	jsonPath := defaultJSONConfigPath
	if path, ok := s.Var("config"); ok {
		jsonPath = path
	}

	bytes, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		s.Fatalf("Failed to read %v: %v", jsonPath, err)
	}

	var testCfg []map[string]string
	if err := json.Unmarshal(bytes, &testCfg); err != nil {
		s.Fatalf("Failed to parse %v in JSON: %v", jsonPath, err)
	}

	for i, cfg := range testCfg {
		s.Logf("Testing camera #%v", i)

		var args []string
		for k, v := range cfg {
			if strings.ContainsAny(k, "=") {
				s.Errorf("Invalid test argument %q", k)
				continue
			}
			args = append(args, fmt.Sprintf("--%v=%v", k, v))
		}
		// Enable verbose logs.
		args = append(args, "--v=1")

		logFileName := fmt.Sprintf("%v_%v.log", gtestExecutableName, i)
		t := gtest.New(gtestExecutableName,
			gtest.ExtraArgs(args...),
			gtest.Logfile(filepath.Join(s.OutDir(), logFileName)))
		if _, err := t.Run(ctx); err != nil {
			s.Errorf("Failed to run %v: %v", gtestExecutableName, err)
		}
	}
}
