// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: GBM,
		Desc: "Exercises the GBM implementation via native tests",
		Contacts: []string{
			"marcheu@chromium.org",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr: []string{"informational"},
	})
}

func GBM(ctx context.Context, s *testing.State) {
	const exec = "/usr/local/libexec/graphics_tests/graphics.GBM.gbmtest"

	list, err := gtest.ListTests(ctx, exec)
	if err != nil {
		s.Fatal("Failed to list gtest: ", err)
	}
	logdir := filepath.Join(s.OutDir(), "graphics.GBM.gbmtest")
	for _, testcase := range list {
		s.Log("Running ", testcase)
		logfile := filepath.Join(logdir, testcase+".log")
		if err := gtest.RunOnce(ctx, exec, testcase, logfile); err != nil {
			s.Errorf("Failed %s. Please find %s for details: %v", testcase, logfile, err)
		}
	}
}
