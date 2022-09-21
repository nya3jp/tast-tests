// Copyright 2020 The ChromiumOS Authors
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
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Nebraska struct hold Nebraska server runtime information.
type Nebraska struct {
	URL string
	cmd *testexec.Cmd
}

// Start starts the nebraska server and returns the Nebraska struct on a
// successful bringup, otherwise an error is returned.
func Start(ctx context.Context) (*Nebraska, error) {
	cmd := testexec.CommandContext(ctx, "nebraska.py",
		"--runtime-root", "/tmp/nebraska",
		"--install-metadata", "/usr/local/dlc",
		"--install-payloads-address", "file:///usr/local/dlc")
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start Nebraska")
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
		if _, err := os.Stat("/tmp/nebraska/port"); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 5}); err != nil {
		return nil, errors.Wrap(err, "Nebraska did not start")
	}

	port, err := ioutil.ReadFile("/tmp/nebraska/port")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the Nebraska's port file")
	}

	success = true
	return &Nebraska{fmt.Sprintf("http://127.0.0.1:%s/update?critical_update=True", string(port)), cmd}, nil
}

// Stop stops nebraska, should pass in the Nebraska struct returned from Start.
func (n *Nebraska) Stop(s *testing.State, name string) error {
	s.Log("Stopping Nebraska")
	// Kill the Nebraska. with SIGINT so it has time to remove port/pid files
	// and cleanup properly.
	n.cmd.Signal(unix.SIGINT)
	n.cmd.Wait()

	if !s.HasError() {
		return nil
	}

	// Read nebraska log and dump it out.
	if b, err := ioutil.ReadFile("/tmp/nebraska.log"); err != nil {
		return errors.Wrap(err, "Nebraska log does not exist")
	} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), name+"-nebraska.log"), b, 0644); err != nil {
		return errors.Wrap(err, "failed to write nebraska log")
	}
	return nil
}
