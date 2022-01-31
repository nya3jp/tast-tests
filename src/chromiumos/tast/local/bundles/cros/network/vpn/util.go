// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// checkPidExists returns if the process with the pid stored in |pidFile| is
// still running. Returns false if |pidFile| does not exist.
func checkPidExists(pidFile string) (bool, error) {
	pidStr, err := ioutil.ReadFile(pidFile)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	pid, err := strconv.Atoi(strings.TrimRight(string(pidStr), "\n"))
	if err != nil {
		return false, err
	}
	process, err := os.FindProcess(int(pid))
	if err != nil {
		return false, errors.Wrapf(err, "failed to find process: %d", pid)
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil, nil
}

// waitForCharonStop waits until the charon process stopped after the
// disconnection of an strongswan-based connection. Otherwise, the next
// connecting attempt (perhaps in another subtest) may fail if the charon is
// still running at that time. If the connection is not strongswan-based,
// returns immediately.
func waitForCharonStop(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		isRunning, err := checkPidExists("/run/ipsec/charon.pid")
		if err != nil {
			return errors.Wrap(err, "failed to check process by pid file")
		}
		if !isRunning {
			return nil
		}
		return errors.New("charon is still running")
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// VerifyVPNProfile verifies a VPN service with certain GUID exists in shill,
// and can be connected if |verifyConnect| set to true.
func VerifyVPNProfile(ctx context.Context, m *shill.Manager, serviceGUID string, verifyConnect bool) error {
	testing.ContextLog(ctx, "Trying to find service with guid ", serviceGUID)

	findServiceProps := make(map[string]interface{})
	findServiceProps["GUID"] = serviceGUID
	findServiceProps["Type"] = "vpn"
	service, err := m.WaitForServiceProperties(ctx, findServiceProps, 5*time.Second)
	if err != nil {
		testing.ContextLog(ctx, "Cannot find service matching guid ", serviceGUID)
		return err
	}
	testing.ContextLogf(ctx, "Found service %v matching guid %s", service, serviceGUID)

	if !verifyConnect {
		return nil
	}

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)
	if err := service.Connect(ctx); err != nil {
		return errors.Wrapf(err, "failed to connect the service %v", service)
	}
	defer func() {
		if err = service.Disconnect(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to disconnect service ", service)
		}
		if err := waitForCharonStop(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop charon: ", err)
		}
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	state, err := pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, append(shillconst.ServiceConnectedStates, shillconst.ServiceStateFailure))
	if err != nil {
		return err
	}

	if state == shillconst.ServiceStateFailure {
		return errors.Errorf("service %v became failure state", service)
	}
	return nil
}
