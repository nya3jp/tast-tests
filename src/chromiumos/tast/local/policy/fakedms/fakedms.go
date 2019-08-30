// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fakedms implements a library for setting policies via a locally-hosted
// Device Management Server.
package fakedms

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Necessary dependencies (as defined in policy-testserver ebuild).
const depsDir = "/usr/local/share/policy_testserver/"

var testserverPath = filepath.Join(depsDir, "policy_testserver.py")
var testserverPythonImports = []string{
	depsDir,
	filepath.Join(depsDir, "tlslite"),
	filepath.Join(depsDir, "testserver"),
	filepath.Join(depsDir, "proto_bindings"),
}

// A FakeDMS struct contains information about a running policy_testserver instance.
type FakeDMS struct {
	cmd        *testexec.Cmd // fakedms process
	URL        string        // fakedms url; needs to be passed to Chrome; set in start()
	policyPath string        // where policies are written for server to read
	pipePath   string        // where the server will write host/port info
	cmdDone    chan struct{} // channel that is closed when Wait() completes
}

// New creates and starts a fake Domain Management Server to serve policies.
// outDir is used to write logs and policies, and should either be in a
// temporary location (and deleted by caller) or in the test's results directory.
func New(ctx context.Context, outDir string) (*FakeDMS, error) {
	if _, err := os.Stat(depsDir); err != nil {
		// Do not try to start server command if it will immediately fail.
		return nil, errors.Wrap(err, "cannot find necessary dependencies folder: "+depsDir)
	}

	policyPath := filepath.Join(outDir, "policy.json")
	logPath := filepath.Join(outDir, "fakedms.log")
	pipePath := filepath.Join(outDir, "startup-pipe.log")

	f, err := os.Create(pipePath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create startup-pipe file")
	}
	defer func() {
		if err := f.Close(); err != nil {
			testing.ContextLog(ctx, "Could not close startup-pipe file: ", err)
		}
	}()

	// TODO(crbug.com/999751): Let caller specify --client-state.
	args := []string{
		testserverPath,
		"--config-file", policyPath,
		"--log-file", logPath,
		"--log-level", "DEBUG",
		"--startup-pipe", "3", // cmd.ExtraFiles: "entry i becomes file descriptor 3+i"
	}
	cmd := testexec.CommandContext(ctx, "python", args...)

	// Add necessary imports to the server command's PYTHONPATH.
	newPP := strings.Join(testserverPythonImports, ":")
	cmd.Env = append(cmd.Env, "PYTHONPATH="+newPP)
	cmd.ExtraFiles = append(cmd.ExtraFiles, f)

	fdms := &FakeDMS{
		cmd:        cmd,
		policyPath: policyPath,
		pipePath:   pipePath,
		cmdDone:    make(chan struct{}, 1),
	}

	if err = fdms.start(ctx); err != nil {
		return nil, err
	}
	return fdms, nil
}

// Ping pings the running FakeDMS server and returns an error if all is not well.
func (fdms *FakeDMS) Ping(ctx context.Context) error {
	resp, err := http.Get(fdms.URL + "/test/ping")
	if err != nil {
		return errors.Wrap(err, "ping request failed")
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.Errorf("ping gave %d response", resp.StatusCode)
	}
	return nil
}

// start runs the FakeDMS and verifies that it is alive.
func (fdms *FakeDMS) start(ctx context.Context) error {
	if err := fdms.cmd.Start(); err != nil {
		return errors.Wrap(err, "FakeDMS start command failed: ")
	}

	go func() {
		if err := fdms.cmd.Wait(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "FakeDMS server stopped unexpectedly: ", err)
		}
		close(fdms.cmdDone)
	}()

	// Read from startup-pipe to determine when the server has completed.
	// example contents: $^@^@^@{"host": "127.0.0.1", "port": 34051}
	// Note that the leading 4 characters are skipped when decoding.
	addr := struct {
		Host string `json:host`
		Port int    `json:port`
	}{}
	pipePoll := func(ctx context.Context) error {
		b, err := ioutil.ReadFile(fdms.pipePath)
		if err != nil {
			return errors.Wrap(err, "could not read from startup-pipe")
		}
		if len(b) == 0 {
			return errors.New("startup-pipe file was empty")
		}
		if b[len(b)-1] != '}' {
			return errors.New("startup-pipe contents did not end in '}'")
		}

		if err = json.Unmarshal(b[4:], &addr); err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not decode startup-pipe"))
		}
		return nil
	}

	startupDone := make(chan error, 1)
	startupCtx, startupCancel := context.WithCancel(ctx)
	defer startupCancel()

	go func() {
		startupDone <- testing.Poll(
			startupCtx, pipePoll, &testing.PollOptions{Timeout: 5 * time.Second})
	}()

	// Wait for server to write host/port info or to exit prematurely.
	select {
	case <-fdms.cmdDone:
		return errors.New("FakeDMS command exited early")
	case err := <-startupDone:
		// Polling finished; kill the command and return if there was an error.
		if err != nil {
			fdms.kill(ctx)
			return errors.Wrap(err, "could not get host/port info")
		}
	case <-ctx.Done():
		fdms.kill(ctx)
		return errors.New("test has timed out")
	}

	fdms.URL = fmt.Sprintf("http://%s:%d", addr.Host, addr.Port)
	testing.ContextLog(ctx, "FakeDMS is up and running on ", fdms.URL)

	return nil
}

// kill will kill the running cmd and wait for it to return.
// To be used only when other errors have occurred; otherwise use Stop().
func (fdms *FakeDMS) kill(ctx context.Context) {
	if err := fdms.cmd.Kill(); err != nil {
		testing.ContextLog(ctx, "Kill command failed: ", err)
	}
	<-fdms.cmdDone
}

// Stop will stop the FakeDMS and return once the command has exited.
func (fdms *FakeDMS) Stop(ctx context.Context) {
	resp, err := http.Get(fdms.URL + "/configuration/test/exit")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			// FakeDMS will exit on its own.
			select {
			case <-fdms.cmdDone:
				testing.ContextLog(ctx, "FakeDMS is closed")
				return
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
			}
		}
	}

	// FakeDMS will not exit on its own.
	fdms.kill(ctx)
}
