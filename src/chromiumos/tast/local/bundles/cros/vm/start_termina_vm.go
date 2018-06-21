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
		Attr:         []string{"informational"},
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func StartTerminaVM(s *testing.State) {
	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	err = vm.SetUpComponent(s.Context(), vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	_, _, c, err := vm.NewDefaultContainer(s.Context(), cr.User(), vm.LiveImageServer)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}

	cmd := c.Command(s.Context(), "pwd")
	_, err = cmd.Output()
	if err != nil {
		cmd.DumpLog(s.Context())
		s.Error("Failed to run pwd: ", err)
	}
}
