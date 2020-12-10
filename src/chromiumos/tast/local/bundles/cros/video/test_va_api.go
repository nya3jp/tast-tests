// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// testVaapiConfig will be used to describe the config used
// to run test_va_api, if needed in the future.
type testVaapiConfig struct {
}

func init() {
	testing.AddTest(&testing.Test{
		Func: TestVAAPI,
		Desc: "Verifies test_va_api in libva-utils",
		Contacts: []string{
			"stevecho@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name: "test_va_api",
			Val:  testVaapiConfig{},
		}},
	})
}

// TestVAAPI runs a set of 17K+ tests related with VA API.
func TestVAAPI(ctx context.Context, s *testing.State) {
	// Execute the test binary.
	const exec = "test_va_api"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		} else {
			s.Error("No additional information is available for this failure")
		}
	}
}
