// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StartTerminaVM,
		Desc: "Checks that a Termina VM starts up with concierge",
		Attr: []string{"bvt"},
	})
}

func StartTerminaVM(s *testing.State) {
	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	concierge, err := vm.New(s.Context(), cr)
	if err != nil {
		s.Fatal("Failed to start concierge: ", err)
	}

	err = concierge.StartTerminaVM(s.Context(), "TestVM")
	if err != nil {
		s.Fatal("Failed to start VM: ", err)
	}
}
