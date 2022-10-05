// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the control of our daemons.
It is meant to have our own implementation so we can support the control in
both local and local test; also, we also wait for D-Bus interfaces to be responsive
instead of only (re)starting them.

Reference code:
src/platform/tast-tests/src/chromiumos/tast/local/upstart/upstart.go
*/

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// DaemonGoal describes a job's goal. See Section 10.1.6.19, "initctl status", in the Upstart Cookbook.
type DaemonGoal string

// DaemonState describes a job's current state. See Section 4.1.2, "Job States", in the Upstart Cookbook.
type DaemonState string

const (
	// unknownGoal indicates that a task job doesn't exist.
	unknownGoal DaemonGoal = "unknown"
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
	// tmpfilesState is a state added on CrOS. See https://crbug.com/1235329.
	tmpfilesState DaemonState = "tmpfiles"
)

var (
	// Matches a leading line of e.g. "ui start/running, process 3182", "ui start/tmpfiles, (tmpfiles) process 3182" or "boot-splash stop/waiting".
	statusRegexp = regexp.MustCompile(`(?m)^[^ ]+ ([-a-z]+)/([-a-z]+)(?:, (?:\([-a-z]+\) )?process (\d+))?$`)

	// A set of all daemon goals
	allGoals = map[DaemonGoal]struct{}{
		unknownGoal: {},
		startGoal:   {},
		stopGoal:    {},
	}

	// A set of all daemon states
	allStates = map[DaemonState]struct{}{
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
		tmpfilesState:  {},
	}
)

// DaemonInfo represents the information for a daemon.
type DaemonInfo struct {
	Name       string
	DaemonName string
	HasDBus    bool
	DBusName   string
	Optional   bool
}

// AttestationDaemon represents the DaemonsInfo for attestation.
var AttestationDaemon = &DaemonInfo{
	Name:       "attestation",
	DaemonName: "attestationd",
	HasDBus:    true,
	DBusName:   "org.chromium.Attestation",
}

// CryptohomeDaemon represents the DaemonsInfo for cryptohome.
var CryptohomeDaemon = &DaemonInfo{
	Name:       "cryptohome",
	DaemonName: "cryptohomed",
	HasDBus:    true,
	DBusName:   "org.chromium.UserDataAuth",
}

// TPMManagerDaemon represents the DaemonsInfo for tpm_manager.
var TPMManagerDaemon = &DaemonInfo{
	Name:       "tpm_manager",
	DaemonName: "tpm_managerd",
	HasDBus:    true,
	DBusName:   "org.chromium.TpmManager",
}

// TrunksDaemon represents the DaemonsInfo for trunks.
var TrunksDaemon = &DaemonInfo{
	Name:       "trunks",
	DaemonName: "trunksd",
	HasDBus:    true,
	DBusName:   "org.chromium.Trunks",
	Optional:   true,
}

// TcsdDaemon represents the DaemonsInfo for tcsd.
var TcsdDaemon = &DaemonInfo{
	Name:       "tcsd",
	DaemonName: "tcsd",
	HasDBus:    false,
	Optional:   true,
}

// PCAAgentDaemon represents the DaemonsInfo for pca_agent.
var PCAAgentDaemon = &DaemonInfo{
	Name:       "pca_agent",
	DaemonName: "pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// FakePCAAgentDaemon represents the DaemonsInfo for fake_pca_agent.
// Note that fake_pca_agentd runs the same service as pca_agentd.
var FakePCAAgentDaemon = &DaemonInfo{
	Name:       "fake_pca_agent",
	DaemonName: "fake_pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// ChapsDaemon represents the DaemonsInfo for chaps.
var ChapsDaemon = &DaemonInfo{
	Name:       "chaps",
	DaemonName: "chapsd",
	HasDBus:    true,
	DBusName:   "org.chromium.Chaps",
}

// BootLockboxDaemon represents the DaemonsInfo for bootlockbox.
var BootLockboxDaemon = &DaemonInfo{
	Name:       "bootlockbox",
	DaemonName: "bootlockboxd",
	HasDBus:    true,
	DBusName:   "org.chromium.BootLockbox",
	Optional:   true,
}

// U2fdDaemon represents the DaemonsInfo for u2fd.
var U2fdDaemon = &DaemonInfo{
	Name:       "u2fd",
	DaemonName: "u2fd",
	HasDBus:    false,
	Optional:   true,
}

// UIDaemon represents the DaemonsInfo for ui.
var UIDaemon = &DaemonInfo{
	Name:       "ui",
	DaemonName: "ui",
	HasDBus:    false,
}

// TPM2SimulatorDaemon represents the DaemonsInfo for tpm2 simulator.
var TPM2SimulatorDaemon = &DaemonInfo{
	Name:       "tpm2-simulator",
	DaemonName: "tpm2-simulator",
	HasDBus:    false,
}

// LowLevelTPMDaemons represents the low level TPM daemons.
var LowLevelTPMDaemons = []*DaemonInfo{
	TcsdDaemon,
	TrunksDaemon,
}

// HighLevelTPMDaemons represents the high level TPM daemons.
var HighLevelTPMDaemons = []*DaemonInfo{
	TPMManagerDaemon,
	ChapsDaemon,
	BootLockboxDaemon,
	PCAAgentDaemon,
	AttestationDaemon,
	U2fdDaemon,
	CryptohomeDaemon,
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
	return dc.waitForDBusService(ctx, CryptohomeDaemon)
}

func (dc *DaemonController) waitForDBusService(ctx context.Context, info *DaemonInfo) error {
	// Create a 30 seconds timeout to wait for D-Bus service.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	name := info.DBusName
	if _, err := dc.r.Run(ctx, "gdbus", "wait", "--system", name); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", name)
	}
	return nil
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
		return errors.Wrapf(err, "failed to stop %s", info.Name)
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

// Status returns the status of daemon.
func (dc *DaemonController) Status(ctx context.Context, info *DaemonInfo) (goal DaemonGoal, state DaemonState, pid int, err error) {
	out, err := dc.r.Run(ctx, "status", info.DaemonName)
	if err != nil {
		if info.Optional {
			// Don't return error if this is an optional daemon.
			return unknownGoal, waitingState, -1, nil
		}
		return unknownGoal, waitingState, -1, errors.Wrap(err, "failed to execute status command")
	}
	return parseStatus(info.DaemonName, string(out))
}

// TryStop stops a daemon if it exist and started.
func (dc *DaemonController) TryStop(ctx context.Context, info *DaemonInfo) error {
	goal, _, _, err := dc.Status(ctx, info)
	if err != nil {
		return errors.Wrapf(err, "failed to get the status of %s", info.Name)
	}
	if goal == startGoal {
		if _, err := dc.r.Run(ctx, "stop", info.DaemonName); err != nil {
			return errors.Wrapf(err, "failed to stop %s", info.Name)
		}
	}
	return nil
}

// Ensure ensures a daemon is started and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Ensure(ctx context.Context, info *DaemonInfo) error {
	goal, _, _, err := dc.Status(ctx, info)
	if err != nil {
		return errors.Wrapf(err, "failed to get the status of %s", info.Name)
	}
	if goal == stopGoal {
		if _, err := dc.r.Run(ctx, "start", info.DaemonName); err != nil {
			return errors.Wrapf(err, "failed to start %s", info.Name)
		}
	}
	if goal != unknownGoal && info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}

// TryStopDaemons tries to stop daemons in the reverse order.
func (dc *DaemonController) TryStopDaemons(ctx context.Context, daemons []*DaemonInfo) error {
	for i := len(daemons) - 1; i >= 0; i-- {
		info := daemons[i]
		if err := dc.TryStop(ctx, info); err != nil {
			return errors.Wrapf(err, "failed to try to stop %s", info.Name)
		}
	}
	return nil
}

// EnsureDaemons ensures daemons started in order.
func (dc *DaemonController) EnsureDaemons(ctx context.Context, daemons []*DaemonInfo) error {
	for _, info := range daemons {
		if err := dc.Ensure(ctx, info); err != nil {
			return errors.Wrapf(err, "failed to ensure %s", info.Name)
		}
	}
	return nil
}

// RestartTPMDaemons restarts all TPM-related daemons.
func (dc *DaemonController) RestartTPMDaemons(ctx context.Context) error {
	if err := dc.TryStopDaemons(ctx, HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := dc.TryStopDaemons(ctx, LowLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
	if err := dc.EnsureDaemons(ctx, LowLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to ensure low-level TPM daemons")
	}
	if err := dc.EnsureDaemons(ctx, HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to ensure high-level TPM daemons")
	}
	return nil
}

// parseStatus parses the output from "initctl status <job>", e.g. "ui start/running, process 28515".
// The output may be multiple lines; see the example in Section 10.1.6.19.3,
// "Single Job Instance Running with Multiple PIDs", in the Upstart Cookbook.
func parseStatus(job, out string) (goal DaemonGoal, state DaemonState, pid int, err error) {
	if !strings.HasPrefix(out, job+" ") {
		return goal, state, pid, errors.Errorf("missing job prefix %q in %q", job, out)
	}
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
