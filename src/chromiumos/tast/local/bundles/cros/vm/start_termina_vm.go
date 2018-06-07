// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartTerminaVM,
		Desc:         "Checks that a Termina VM starts up with concierge, and a container starts in that VM",
		Timeout:      300 * time.Second,
		Attr:         []string{"bvt"},
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func StartTerminaVM(s *testing.State) {
	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	concierge, err := vm.New(s.Context(), cr.User())
	if err != nil {
		s.Fatal("Failed to start concierge: ", err)
	}

	err = concierge.StartTerminaVM(s.Context())
	if err != nil {
		s.Fatal("Failed to start VM: ", err)
	}

	err = concierge.StartContainer(s.Context())
	if err != nil {
		s.Fatal("Failed to start Container: ", err)
	}
}
