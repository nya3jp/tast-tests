// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Clvk,
		Desc: "Run OpenCL implementation on top of Vulkan using clvk",
		Contacts: []string{
			"rjodin@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vulkan"},
		Fixture:      "graphicsNoChrome",
		Timeout:      3 * time.Minute,
	})
}

func Clvk(ctx context.Context, s *testing.State) {
	err := testexec.CommandContext(ctx, "/usr/local/opencl_tests/api_tests").Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("clvk/api_tests test failed: ", err)
	}
	err = testexec.CommandContext(ctx, "/usr/local/opencl_tests/simple_test").Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("clvk/simple_test test failed: ", err)
	}
}
