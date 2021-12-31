// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemfwdSmoke,
		Desc:         "Verifies that modemfwd initializes without errors",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		Fixture:      "cellular",
		SoftwareDeps: []string{"modemfwd"},
		Timeout:      2 * time.Minute,
	})
}

// ModemfwdSmoke Test
func ModemfwdSmoke(ctx context.Context, s *testing.State) {
	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}

	if fileExists(modemfwd.DisableAutoUpdatePref) {
		os.Remove(modemfwd.DisableAutoUpdatePref)
		s.Fatalf("%q file found. This file was not properly removed in another test. Deleting it now", modemfwd.DisableAutoUpdatePref)
	}

	// modemfwd is initially stopped in the fixture SetUp
	if err := modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
	s.Log("modemfwd has started successfully")
	if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
		s.Fatalf("Failed to stop %q: %s", modemfwd.JobName, err)
	}
	s.Log("modemfwd has stopped successfully")
}
