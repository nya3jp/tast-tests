// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dawn,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that Dawn unit and end-to-end tests run successfully",
		Contacts: []string{
			"hob@chromium.org",
			"chromeos-gfx@google.com",
		},
		Params: []testing.Param{{
			Name: "unit_tests",
			Val:  "dawn_unittests",
		}, {
			Name: "end_to_end_tests",
			Val:  "dawn_end2end_tests",
		}},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "gpuWatchDog",
	})
}

func Dawn(ctx context.Context, s *testing.State) {
	exec := filepath.Join("/usr/local/libexec/chrome-binary-tests/", s.Param().(string))
	list, err := gtest.ListTests(ctx, exec)
	if err != nil {
		s.Fatal("Failed to list gtest: ", err)
	}
	logdir := filepath.Join(s.OutDir(), "gtest")
	for _, testcase := range list {
		s.Log("Running ", testcase)
		if _, err := gtest.New(exec,
			gtest.Logfile(filepath.Join(logdir, testcase+".log")),
			gtest.Filter(testcase),
		).Run(ctx); err != nil {
			s.Errorf("%s failed: %v", testcase, err)
		}
	}
}
