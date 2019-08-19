// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DawnUnittests,
		Desc: "Verifies dawn_unittests runs successfully",
		Contacts: []string{
			"hob@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      1 * time.Minute,
	})
}

func DawnUnittests(ctx context.Context, s *testing.State) {
	const exec = "/usr/libexec/chrome-binary-tests/dawn_unittests"
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
