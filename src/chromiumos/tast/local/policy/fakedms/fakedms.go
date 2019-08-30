// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fakedms

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	URL        string        // fakedms url; needs to be passed to Chrome
	policyPath string        // where policies are written for server to read
	logPath    string        // where policy_testserver logs are written
	cmdDone    chan struct{} // channel that is closed when Wait() completes
}

// getUnusedLocalPort temporarily opens a tcp listener to find an unused port.
// WARNING: there is a potential race condition, as this function does not
// guarantee that another process does not also pick the port before use.
func getUnusedLocalPort() (int, error) {
	tl, err := net.ListenTCP("tcp", &net.TCPAddr{IP: []byte{127, 0, 0, 1}})
	if err != nil {
		return 0, errors.Wrap(err, "could not open tcp listener: ")
	}
	defer tl.Close()
	tcpAddr, err := net.ResolveTCPAddr("tcp", tl.Addr().String())
	if err != nil {
		return 0, errors.Wrap(err, "could not resolve tcp addr: ")
	}
	return tcpAddr.Port, nil
}

// New creates and starts a fake Domain Management Server to serve policies.
// outDir is used to write logs and policies, and should either be in a
// temporary location (and deleted by caller) or in the test's results directory.
func New(ctx context.Context, outDir string) (*FakeDMS, error) {
	if _, err := os.Stat(depsDir); err != nil {
		// Do not try to start server command if it will immediately fail.
		return nil, errors.Wrap(err, "cannot find necessary dependencies folder: "+depsDir)
	}

	fdms := &FakeDMS{
		policyPath: filepath.Join(outDir, "policy.json"),
		logPath:    filepath.Join(outDir, "fakedms.log"),
		cmdDone:    make(chan struct{}),
	}

	port, err := getUnusedLocalPort()
	if err != nil {
		return nil, errors.Wrap(err, "could not find unused port")
	}
	const localhost = "127.0.0.1"
	fdms.URL = fmt.Sprintf("http://%s:%d", localhost, port)

	args := []string{
		testserverPath,
		"--config-file", fdms.policyPath,
		"--host", localhost,
		"--port", strconv.Itoa(port),
		"--log-file", fdms.logPath,
		"--log-level", "DEBUG",
	}
	fdms.cmd = testexec.CommandContext(ctx, "python", args...)

	// Add necessary imports to the server command's PYTHONPATH.
	newPP := strings.Join(testserverPythonImports, ":")
	fdms.cmd.Env = append(fdms.cmd.Env, "PYTHONPATH="+newPP)

	return fdms, fdms.start(ctx)
}

// Ping pings the running FakeDMS server and returns an error if all is not well.
func (fdms *FakeDMS) Ping(ctx context.Context) error {
	resp, err := http.Get(fdms.URL + "/test/ping")
	if err != nil {
		return errors.Wrap(err, "ping failed: ")
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.Errorf("ping gave %d response", resp.StatusCode)
	}
	return nil
}

// start runs the FakeDMS and verifies that it is alive.
func (fdms *FakeDMS) start(ctx context.Context) error {
	err := fdms.cmd.Start()
	if err != nil {
		fdms.cmd.DumpLog(ctx)
		return errors.Wrap(err, "FakeDMS start command failed: ")
	}

	go func() {
		if fdms.cmd.Wait() != nil {
			testing.ContextLog(ctx, "FakeDMS server has stopped unexpectedly")
			fdms.cmd.DumpLog(ctx)
		}
		close(fdms.cmdDone)
	}()

	pingDone := make(chan error)
	pingCtx, pingCancel := context.WithCancel(ctx)
	go func() {
		pingDone <- testing.Poll(
			pingCtx, fdms.Ping, &testing.PollOptions{Timeout: 5 * time.Second})
	}()

	// Wait for either a successful Ping or for the server to exit prematurely.
	select {
	case <-fdms.cmdDone:
		// Command exited early; cancel the poll and return.
		pingCancel()
		<-pingDone
		return errors.New("FakeDMS command exited early")
	case err := <-pingDone:
		// Polling finished; kill the command and return if there was an error.
		if err != nil {
			fdms.cmd.Kill()
			<-fdms.cmdDone
			return err
		}
	}
	testing.ContextLog(ctx, "FakeDMS is up and running on ", fdms.URL)
	return nil
}

// Stop will stop the FakeDMS and return once the command has exited.
func (fdms *FakeDMS) Stop(ctx context.Context) {
	resp, err := http.Get(fmt.Sprintf("%s/configuration/test/exit", fdms.URL))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			// FakeDMS will exit on its own.
			select {
			case <-fdms.cmdDone:
				testing.ContextLog(ctx, "FakeDMS is closed")
				return
			case <-time.After(1 * time.Second):
			}
		}
	}

	// FakeDMS will not exit on its own; kill it and then wait on Wait().
	fdms.cmd.Kill()
	<-fdms.cmdDone
}
