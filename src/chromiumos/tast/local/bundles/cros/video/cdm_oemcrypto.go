// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CDMOEMCrypto,
		Desc: "Verifies that Widevine CE CDM and OEMCrypto tests run successfully",
		Contacts: []string{
			"jkardatzke@google.com",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"protected_content"},
		Timeout:      15 * time.Minute,
		Attr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		Params: []testing.Param{{
			Name: "ce_cdm",
			Val:  "widevine_ce_cdm_hw_tests",
		}, {
			Name: "oemcrypto",
			Val:  "oemcrypto_hw_ref_tests",
		}},
	})
}

func CDMOEMCrypto(ctx context.Context, s *testing.State) {
	testExec := s.Param().(string)
	logdir := filepath.Join(s.OutDir(), "gtest")
	s.Log("Running ", testExec)
	if _, err := gtest.New(testExec,
		gtest.Logfile(filepath.Join(logdir, testExec+".log")),
		gtest.Filter("-*Huge*"),
	).Run(ctx); err != nil {
		s.Errorf("%s failed: %v", testExec, err)
	}
}
