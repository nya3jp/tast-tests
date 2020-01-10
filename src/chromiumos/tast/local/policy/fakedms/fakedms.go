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
	"io"
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
	done       chan struct{} // channel that is closed when Wait() completes
	policyPath string        // where policies are written for server to read
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

	fr, fw, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create startup-pipe file")
	}
	defer func() {
		if err := fr.Close(); err != nil {
			testing.ContextLog(ctx, "Could not close startup-pipe read file: ", err)
		}
		if err := fw.Close(); err != nil {
			testing.ContextLog(ctx, "Could not close startup-pipe write file: ", err)
		}
	}()

	// TODO(crbug.com/999751): Let caller specify --client-state.
	args := []string{
		testserverPath,
		"--config-file", policyPath,
		"--log-file", logPath,
		"--log-level", "DEBUG",
		// cmd.ExtraFiles (set below) assigns element i to file descriptor 3+i.
		// See exec.Cmd for more info.
		"--startup-pipe", "3",
	}
	cmd := testexec.CommandContext(ctx, "python", args...)

	// Add necessary imports to the server command's PYTHONPATH.
	newPP := strings.Join(testserverPythonImports, ":")
	cmd.Env = append(cmd.Env, "PYTHONPATH="+newPP)
	cmd.ExtraFiles = []*os.File{fw}

	fdms := &FakeDMS{
		cmd:        cmd,
		done:       make(chan struct{}, 1),
		policyPath: policyPath,
	}

	if err = fdms.start(ctx, fr); err != nil {
		return nil, err
	}
	return fdms, nil
}

// start runs the FakeDMS and verifies that it is alive.
// p is a pipe reader created and passed in by New().
func (fdms *FakeDMS) start(ctx context.Context, p *os.File) error {
	if err := fdms.cmd.Start(); err != nil {
		return errors.Wrap(err, "FakeDMS start command failed")
	}

	go func() {
		if err := fdms.cmd.Wait(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "FakeDMS server stopped unexpectedly: ", err)
		}
		close(fdms.done)
	}()

	// Read from the startup-pipe to see when the server has completed startup.
	// example contents: $^@^@^@{"host": "127.0.0.1", "port": 34051}
	// Note that the leading 4 characters are skipped when decoding.
	type pResult struct {
		URL string
		Err error
	}
	pDone := make(chan pResult, 1)

	go func() {
		// Ignore the first 4 bytes.
		b4 := make([]byte, 4)
		if _, err := io.ReadFull(p, b4); err != nil {
			pDone <- pResult{Err: errors.Wrap(err, "could not read from startup-pipe")}
			return
		}

		var addr struct {
			Host string
			Port int
		}
		if err := json.NewDecoder(p).Decode(&addr); err != nil {
			pDone <- pResult{Err: errors.Wrap(err, "could not read host/port info")}
			return
		}

		pDone <- pResult{URL: fmt.Sprintf("http://%s:%d", addr.Host, addr.Port)}
	}()

	// Wait for server to write host/port info or to exit prematurely.
	select {
	case <-fdms.done:
		return errors.New("FakeDMS command exited early")
	case <-ctx.Done():
		fdms.kill(ctx)
		return errors.Errorf("test has timed out: %s", ctx.Err())
	case <-time.After(10 * time.Second):
		fdms.kill(ctx)
		return errors.New("FakeDMS took more than 10 seconds to start")
	case p := <-pDone:
		if p.Err != nil {
			fdms.kill(ctx)
			return errors.Wrap(p.Err, "could not get host/port info")
		}
		fdms.URL = p.URL
	}

	testing.ContextLog(ctx, "FakeDMS is up and running on ", fdms.URL)
	return nil
}

// WritePolicyBlob will write the given PolicyBlob to be read by the FakeDMS.
func (fdms *FakeDMS) WritePolicyBlob(pb *PolicyBlob) error {
	pJSON, err := json.Marshal(pb)
	if err != nil {
		return errors.Wrap(err, "could not convert policies to JSON")
	}

	if err = ioutil.WriteFile(fdms.policyPath, pJSON, 0644); err != nil {
		return errors.Wrap(err, "could not write JSON to file")
	}
	return nil
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

// kill will kill the running cmd and wait for it to return.
// To be used only when other errors have occurred; otherwise use Stop().
func (fdms *FakeDMS) kill(ctx context.Context) {
	if err := fdms.cmd.Kill(); err != nil {
		testing.ContextLog(ctx, "Kill command failed: ", err)
	}
	<-fdms.done
}

// Stop will stop the FakeDMS and return once the command has exited.
func (fdms *FakeDMS) Stop(ctx context.Context) {
	resp, err := http.Get(fdms.URL + "/configuration/test/exit")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			// FakeDMS will exit on its own.
			select {
			case <-fdms.done:
				testing.ContextLog(ctx, "FakeDMS is closed")
				return
			case <-time.After(1 * time.Second):
				testing.ContextLog(ctx, "Took more than 1 second to close FakeDMS")
			case <-ctx.Done():
				testing.ContextLog(ctx, "Test timed out: ", ctx.Err())
			}
		}
	}

	// FakeDMS will not exit on its own.
	fdms.kill(ctx)
}
