// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GraphicsDEQP,
		Desc: "Runs the drawElements Quality Program test suite shipped with test images",
		Attr: []string{"disabled"},
	})
}

func DEQP(s *testing.State) {
	// The actual test goes here.
}
