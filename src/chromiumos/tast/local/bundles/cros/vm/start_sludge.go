// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartSludge,
		Desc:         "Checks that sludge VM can start correctly on wilco devices",
		Contacts:     []string{"tbegin@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host, wilco"},
	})
}

// StartSludge starts an instance of sludge VM and tests that the DTC binaries
//  are running. If everything is running correctly, it then shuts down the VM.
func StartSludge(ctx context.Context, s *testing.State) {
	const (
		wilcoVMJob = "wilco_dtc"
		wilcoVMCID = "512"
	)

	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		s.Fatal("unable to stop already running wilco")
	}
	if err := upstart.EnsureJobRunning(ctx, wilcoVMJob); err != nil {
		s.Fatal("wilco DTC process could not start: ", err)
	}

	// Wait for the VM and binaries to stabilize
	testing.Sleep(ctx, 2*time.Second)

	for _, name := range []string{"ddv", "sa"} {
		s.Logf("Checking %v process", name)
		cmd := testexec.CommandContext(ctx,
			"vsh", fmt.Sprintf("--cid=%s", wilcoVMCID), "--", "pgrep", name)
		cmd.Stdin = &bytes.Buffer{}

		if out, err := cmd.CombinedOutput(); err != nil {
			s.Errorf("%v process not found: %v", name, err)
		} else {
			s.Logf("%v process running with PID %v",
				name, strings.TrimSpace(string(out)))
		}
	}

	err := upstart.StopJob(ctx, wilcoVMJob)
	if err != nil {
		s.Error("unable to stop Wilco DTC process")
	}
}
