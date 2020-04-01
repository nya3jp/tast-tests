// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsCloseAllSessions,
		Desc: "Verifies that the behaviour of C_CloseAllSessions() in libchaps is correct",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
	})
}

const (
	delayForLoopRoutine = 700 * time.Millisecond
)

// ChapsCloseAllSessions verifies the behaviour of C_CloseAllSessions() in libchaps is correct.
func ChapsCloseAllSessions(ctx context.Context, s *testing.State) {
	loopErr := make(chan error)
	loopRoutine := func() {
		cmdRunner, err := hwseclocal.NewCmdRunner()
		if err != nil {
			loopErr <- errors.Wrap(err, "failed to create CmdRunner")
			return
		}

		if _, err := cmdRunner.Run(ctx, "p11_replay", "--replay_close_all_sessions", "--use_sessions_loop"); err != nil {
			loopErr <- errors.Wrap(err, "failed to run p11_replay --replay_close_all_sessions --use_sessions_loop")
		} else {
			loopErr <- nil
		}
	}

	checkErr := make(chan error)
	checkRoutine := func() {
		cmdRunner, err := hwseclocal.NewCmdRunner()
		if err != nil {
			checkErr <- errors.Wrap(err, "failed to create CmdRunner")
			return
		}

		// Wait for loop routine to start running first.
		if err := testing.Sleep(ctx, delayForLoopRoutine); err != nil {
			checkErr <- errors.Wrap(err, "interrupted while sleeping")
			return
		}

		if _, err := cmdRunner.Run(ctx, "p11_replay", "--replay_close_all_sessions", "--check_close_all_sessions"); err != nil {
			checkErr <- errors.Wrap(err, "failed to run p11_replay --replay_close_all_sessions --check_close_all_sessions")
		} else {
			checkErr <- nil
		}
	}

	go loopRoutine()
	go checkRoutine()

	if err := <-loopErr; err != nil {
		s.Error("Loop routine failed: ", err)
	}

	if err := <-checkErr; err != nil {
		s.Error("Check routine failed: ", err)
	}
}
