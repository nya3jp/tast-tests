// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Panic,
		Desc: "Panics to demonstrate that other tests should still run",
		Attr: []string{"disabled"},
	})
}

func Panic(s *testing.State) {
	s.Log("About to panic")
	panic("This is an intentional panic")
}
