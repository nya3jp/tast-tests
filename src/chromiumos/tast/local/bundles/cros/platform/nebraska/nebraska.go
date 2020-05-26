// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nebraska provides start/stop functions for nebraska.
package nebraska

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// StartNebraska starts the nebraska server and returns the string url to send
// requests to and testexec.Cmd which is running nebraska.
func StartNebraska(ctx context.Context, s *testing.State) (string, *testexec.Cmd) {
	s.Log("Starting Nebraska")
	cmd := testexec.CommandContext(ctx, "nebraska.py",
		"--runtime-root", "/tmp/nebraska",
		"--install-metadata", "/usr/local/dlc",
		"--install-payloads-address", "file:///usr/local/dlc")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start Nebraska: ", err)
	}

	success := false
	defer func() {
		if success {
			return
		}
		cmd.Kill()
		cmd.Wait()
	}()

	// Try a few times to make sure Nebraska is up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/tmp/nebraska/port"); os.IsNotExist(err) {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 5}); err != nil {
		s.Fatal("Nebraska did not start: ", err)
	}

	port, err := ioutil.ReadFile("/tmp/nebraska/port")
	if err != nil {
		s.Fatal("Failed to read the Nebraska's port file: ", err)
	}

	success = true
	return fmt.Sprintf("http://127.0.0.1:%s/update?critical_update=True", string(port)), cmd
}

// StopNebraska stops nebraska, should pass in the testexec.Cmd returned from
// StartNebraska.
func StopNebraska(s *testing.State, cmd *testexec.Cmd, name string) {
	s.Log("Stopping Nebraska")
	// Kill the Nebraska. with SIGINT so it has time to remove port/pid files
	// and cleanup properly.
	cmd.Signal(syscall.SIGINT)
	cmd.Wait()

	if !s.HasError() {
		return
	}

	// Read nebraska log and dump it out.
	if b, err := ioutil.ReadFile("/tmp/nebraska.log"); err != nil {
		s.Error("Nebraska log does not exist: ", err)
	} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), name+"-nebraska.log"), b, 0644); err != nil {
		s.Error("Failed to write nebraska log: ", err)
	}
}
