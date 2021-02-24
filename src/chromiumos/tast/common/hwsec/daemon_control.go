// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the control of our daemons.
It is meant to have our own implementation so we can support the control in
both local and local test; also, we also wait for D-Bus interfaces to be responsive
instead of only (re)starting them.

And we referenced this code:
src/platform/tast-tests/src/chromiumos/tast/local/upstart/upstart.go
*/

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DaemonGoal describes a job's goal. See Section 10.1.6.19, "initctl status", in the Upstart Cookbook.
type DaemonGoal string

// DaemonState describes a job's current state. See Section 4.1.2, "Job States", in the Upstart Cookbook.
type DaemonState string

const (
	// startGoal indicates that a task or service job has been started.
	startGoal DaemonGoal = "start"
	// stopGoal indicates that a task job has completed or that a service job has been manually stopped or has
	// a "stop on" condition that has been satisfied.
	stopGoal DaemonGoal = "stop"

	// waitingState is the initial state for a job.
	waitingState DaemonState = "waiting"
	// startingState indicates that a job is about to start.
	startingState DaemonState = "starting"
	// securityState indicates that a job is having its AppArmor security policy loaded.
	securityState DaemonState = "security"
	// preStartState indicates that a job's pre-start section is running.
	preStartState DaemonState = "pre-start"
	// spawnedState indicates that a job's script or exec section is about to run.
	spawnedState DaemonState = "spawned"
	// postStartState indicates that a job's post-start section is running.
	postStartState DaemonState = "post-start"
	// runningState indicates that a job is running (i.e. its post-start section has completed). It may not have a PID yet.
	runningState DaemonState = "running"
	// preStopState indicates that a job's pre-stop section is running.
	preStopState DaemonState = "pre-stop"
	// stoppingState indicates that a job's pre-stop section has completed.
	stoppingState DaemonState = "stopping"
	// killedState indicates that a job is about to be stopped.
	killedState DaemonState = "killed"
	// postStopState indicates that a job's post-stop section is running.
	postStopState DaemonState = "post-stop"
)

var allGoals = map[DaemonGoal]struct{}{
	startGoal: {},
	stopGoal:  {},
}

var allStates = map[DaemonState]struct{}{
	waitingState:   {},
	startingState:  {},
	securityState:  {},
	preStartState:  {},
	spawnedState:   {},
	postStartState: {},
	runningState:   {},
	preStopState:   {},
	stoppingState:  {},
	killedState:    {},
	postStopState:  {},
}

// DaemonInfo represents the information for a daemon
type DaemonInfo struct {
	Name       string
	DaemonName string
	HasDBus    bool
	DBusName   string
}

// AttestationDaemonInfo represents the DaemonsInfo for attestation.
var AttestationDaemonInfo = &DaemonInfo{
	Name:       "attestation",
	DaemonName: "attestationd",
	HasDBus:    true,
	DBusName:   "org.chromium.Attestation",
}

// CryptohomeDaemonInfo represents the DaemonsInfo for cryptohome.
var CryptohomeDaemonInfo = &DaemonInfo{
	Name:       "cryptohome",
	DaemonName: "cryptohomed",
	HasDBus:    true,
	DBusName:   "org.chromium.Cryptohome",
}

// TPMManagerDaemonInfo represents the DaemonsInfo for tpm_manager.
var TPMManagerDaemonInfo = &DaemonInfo{
	Name:       "tpm_manager",
	DaemonName: "tpm_managerd",
	HasDBus:    true,
	DBusName:   "org.chromium.TpmManager",
}

// TrunksDaemonInfo represents the DaemonsInfo for trunks.
var TrunksDaemonInfo = &DaemonInfo{
	Name:       "trunks",
	DaemonName: "trunksd",
	HasDBus:    true,
	DBusName:   "org.chromium.Trunks",
}

// TcsdDaemonInfo represents the DaemonsInfo for tcsd.
var TcsdDaemonInfo = &DaemonInfo{
	Name:       "tcsd",
	DaemonName: "tcsd",
	HasDBus:    false,
}

// PCAAgentDaemonInfo represents the DaemonsInfo for pca_agent.
var PCAAgentDaemonInfo = &DaemonInfo{
	Name:       "pca_agent",
	DaemonName: "pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// FakePCAAgentDaemonInfo represents the DaemonsInfo for fake_pca_agent.
// Note that fake_pca_agentd runs the same service as pca_agentd
var FakePCAAgentDaemonInfo = &DaemonInfo{
	Name:       "fake_pca_agent",
	DaemonName: "fake_pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// ChapsDaemonInfo represents the DaemonsInfo for chaps.
var ChapsDaemonInfo = &DaemonInfo{
	Name:       "chaps",
	DaemonName: "chapsd",
	HasDBus:    true,
	DBusName:   "org.chromium.Chaps",
}

// BootLockboxDaemonInfo represents the DaemonsInfo for bootlockbox.
var BootLockboxDaemonInfo = &DaemonInfo{
	Name:       "bootlockbox",
	DaemonName: "bootlockboxd",
	HasDBus:    true,
	DBusName:   "org.chromium.BootLockbox",
}

// U2fdDaemonInfo represents the DaemonsInfo for u2fd.
var U2fdDaemonInfo = &DaemonInfo{
	Name:       "u2fd",
	DaemonName: "u2fd",
	HasDBus:    false,
}

// UIDaemonInfo represents the DaemonsInfo for ui.
var UIDaemonInfo = &DaemonInfo{
	Name:       "ui",
	DaemonName: "ui",
	HasDBus:    false,
}

// DaemonController controls the daemons via upstart commands.
type DaemonController struct {
	r CmdRunner
}

// NewDaemonController creates a new DaemonController object, where r is used to run the command internally.
func NewDaemonController(r CmdRunner) *DaemonController {
	return &DaemonController{r}
}

// WaitForAllDBusServices waits for all D-Bus services of our interest to be running.
func (dc *DaemonController) WaitForAllDBusServices(ctx context.Context) error {
	// Just waits for cryptohomd because it's at the tail of dependency chain. We might have to change it if any dependency is decoupled.
	return dc.waitForDBusService(ctx, CryptohomeDaemonInfo)
}

func (dc *DaemonController) waitForDBusService(ctx context.Context, info *DaemonInfo) error {
	if !info.HasDBus {
		return errors.Errorf("%s doesn't have D-Bus interface", info.Name)
	}
	// Without quote, we might find something prefixed by name.
	name := "\"" + info.DBusName + "\""
	return testing.Poll(ctx, func(ctx context.Context) error {
		if out, err := dc.r.Run(
			ctx,
			"dbus-send",
			"--system",
			"--dest=org.freedesktop.DBus",
			"--print-reply",
			"/org/freedesktop/DBus",
			"org.freedesktop.DBus.ListNames"); err != nil {
			return err
		} else if strings.Contains(string(out), name) {
			return nil
		}
		return errors.New("daemon not up")
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// Start starts a daemon and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Start(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "start", info.DaemonName); err != nil {
		return errors.Wrapf(err, "failed to start %s", info.Name)
	}
	if info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}

// Stop stops a daemon.
func (dc *DaemonController) Stop(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "stop", info.DaemonName); err != nil {
		return errors.Wrap(err, "failed to stop fake_pca_agent")
	}
	return nil
}

// Restart restarts a daemon and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Restart(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "restart", info.DaemonName); err != nil {
		return errors.Wrapf(err, "failed to restart %s", info.Name)
	}
	if info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}

// parseStatus parses the output from "initctl status <job>", e.g. "ui start/running, process 28515".
// The output may be multiple lines; see the example in Section 10.1.6.19.3,
// "Single Job Instance Running with Multiple PIDs", in the Upstart Cookbook.
func (dc *DaemonController) parseStatus(job, out string) (goal DaemonGoal, state DaemonState, pid int, err error) {
	if !strings.HasPrefix(out, job+" ") {
		return goal, state, pid, errors.Errorf("missing job prefix %q in %q", job, out)
	}
	// Matches a leading line of e.g. "ui start/running, process 3182" or "boot-splash stop/waiting".
	statusRegexp := regexp.MustCompile(`(?m)^[^ ]+ ([-a-z]+)/([-a-z]+)(?:, process (\d+))?$`)
	m := statusRegexp.FindStringSubmatch(out)
	if m == nil {
		return goal, state, pid, errors.Errorf("unexpected format in %q", out)
	}

	goal = DaemonGoal(m[1])
	if _, ok := allGoals[goal]; !ok {
		return goal, state, pid, errors.Errorf("invalid goal %q", m[1])
	}

	state = DaemonState(m[2])
	if _, ok := allStates[state]; !ok {
		return goal, state, pid, errors.Errorf("invalid state %q", m[2])
	}

	if m[3] != "" {
		p, err := strconv.ParseInt(m[3], 10, 32)
		if err != nil {
			return goal, state, pid, errors.Errorf("bad PID %q", m[3])
		}
		pid = int(p)
	}

	return goal, state, pid, nil
}

// Ensure ensures a daemon is started and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Ensure(ctx context.Context, info *DaemonInfo) error {
	out, err := dc.r.Run(ctx, "status", info.DaemonName)
	if err != nil {
		return errors.Wrapf(err, "failed to get the status of %s", info.Name)
	}
	goal, _, _, err := dc.parseStatus(info.DaemonName, string(out))
	if goal == stopGoal {
		if _, err := dc.r.Run(ctx, "start", info.DaemonName); err != nil {
			return errors.Wrapf(err, "failed to start %s", info.Name)
		}
	}
	if info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}

func (dc *DaemonController) StopAllServices(ctx context.Context) error {
	return nil
}

func (dc *DaemonController) EnsureAllServices(ctx context.Context) error {
	return nil
}
