// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

type execConfig struct {
	exec      string
	extraArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CDMOEMCrypto,
		Desc: "Verifies that Widevine CE CDM and OEMCrypto tests run successfully",
		Contacts: []string{
			"jkardatzke@google.com",
			"chromeos-gfx-video@google.com",
		},
		Params: []testing.Param{{
			Name: "ce_cdm",
			Val:  execConfig{"widevine_ce_cdm_hw_tests", []string{}},
		}, {
			Name: "oemcrypto",
			Val:  execConfig{"oemcrypto_hw_ref_tests", []string{}},
		}},
		// TODO(jkardatzke): Add SoftwareDeps for cdm_factory_daemon USE flag and
		// add Attr to enable this for CI once this is functional.
	})
}

func CDMOEMCrypto(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(execConfig)
	logdir := filepath.Join(s.OutDir(), "gtest")
	s.Log("Running ", testOpt.exec)
	if _, err := gtest.New(testOpt.exec,
		gtest.Logfile(filepath.Join(logdir, testOpt.exec+".log")),
		gtest.ExtraArgs(testOpt.extraArgs...),
	).Run(ctx); err != nil {
		s.Errorf("%s failed: %v", testOpt.exec, err)
	}
}
