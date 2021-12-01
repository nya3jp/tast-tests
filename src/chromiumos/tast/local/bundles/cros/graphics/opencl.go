// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os/exec"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Opencl,
		Desc: "Run clvk api_tests and simple_test",
		Contacts: []string{
			"rjodin@chromium.org", // Tast port author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vulkan"},
		Timeout:      3 * time.Minute,
	})
}

func Opencl(ctx context.Context, s *testing.State) {
	_, err := exec.Command("/usr/local/opencl_tests/api_tests").Output()
	if err != nil {
		s.Fatal("clvk/api_tests test failed: ", err)
	}
	_, err = exec.Command("/usr/local/opencl_tests/simple_test").Output()
	if err != nil {
		s.Fatal("clvk/simple_test test failed: ", err)
	}
}
