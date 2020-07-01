// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"math/rand"
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
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
	})
}

const (
	delayForLoopRoutine = 700 * time.Millisecond
)

// ChapsCloseAllSessions verifies the behaviour of C_CloseAllSessions() in libchaps is correct.
func ChapsCloseAllSessions(ctx context.Context, s *testing.State) {
	// This test works by running --check_close_all_sessions and --use_sessions_loop part of p11_replay concurrently.
	// --use_sessions_loop will create a session for itself, then notify through the IPC file that it's got a working session, then it'll continuously verify that the session is still working, until it've received communication from the IPC file that we're done, then it'll exit.
	// --check_close_all_sessions will wait for --use_sessions_loop to be ready, as in, --use_sessions_loop's session is created. --check_close_all_sessions waits for this even through polling the IPC file. After that, it'll create a session (for itself, different from the one created by --use_sessions_loop), call C_CloseAllSessions() to verify that it is closed, then signal to --use_sessions_loop through IPC file that it's done.
	// In summary, what is done is checking that C_CloseAllSessions() doesn't affect other users of chaps PKCS#11 API, while C_CloseAllSessions() works.

	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	// Create the file for IPC between --check_close_all_sessions and --use_sessions_loop.
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	ipcFile := fmt.Sprintf("/tmp/chaps_close_all_sessions_%d_%d", seededRand.Uint32(), seededRand.Uint32())
	defer func() {
		// We cleanup anyway, but if it failed and cleanup is needed, then we log the warning.
		if _, err := cmdRunner.Run(ctx, "rm", "-f", ipcFile); err != nil {
			s.Log("Failed to remove the IPC file: ", err)
		}
	}()

	checkErr := make(chan error)
	checkRoutine := func() {
		if _, err := cmdRunner.Run(ctx, "p11_replay", "--replay_close_all_sessions", "--check_close_all_sessions", "--ipc_file="+ipcFile); err != nil {
			checkErr <- errors.Wrap(err, "failed to run p11_replay --replay_close_all_sessions --check_close_all_sessions")
		} else {
			checkErr <- nil
		}
	}

	go checkRoutine()

	if _, err := cmdRunner.Run(ctx, "p11_replay", "--replay_close_all_sessions", "--use_sessions_loop", "--ipc_file="+ipcFile); err != nil {
		s.Error("Loop routine failed: ", err)
	}

	if err := <-checkErr; err != nil {
		s.Error("Check routine failed: ", err)
	}
}
