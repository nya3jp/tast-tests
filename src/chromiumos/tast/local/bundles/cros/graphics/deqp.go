// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DEQP,
		Data: []string{"deqp_bvt.txt"},
		Desc: "Runs the drawElements Quality Program test suite shipped with test images",
		Attr: []string{"disabled"},
	})
}

func DEQP(s *testing.State) {
	b, err := ioutil.ReadFile(s.DataPath("deqp_bvt.txt"))
	if err != nil {
		s.Fatal("Could not open the file containing the list of tests: ", err)
	}
	tests := strings.Split(string(b), "\n")
	for i, test := range tests {
		s.Logf("[%d/%d] TestCase: %s", i+1, len(tests), test)
	}
	major, minor, err := graphics.GLESVersion(s.Context())
	vulkan, err := graphics.SupportsVulkanForDEQP(s.Context())
	s.Log(graphics.SupportedAPIs(major, minor, vulkan))
}
