// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LocalPanic,
		Desc: "Helper test that panics",
		// This test is called by remote tests in the meta package.
		Attr: []string{"disabled"},
	})
}

func LocalPanic(s *testing.State) {
	panic("intentionally panicking")
}
