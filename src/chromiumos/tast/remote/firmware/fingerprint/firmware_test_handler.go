// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"path/filepath"
	"time"

	fp "chromiumos/tast/common/fingerprint"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/remote/sysutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// FirmwareTest provides a common framework for fingerprint firmware tests.
type FirmwareTest struct {
	d                        *rpcdut.RPCDUT
	servo                    *servo.Proxy
	fpBoard                  fp.BoardName
	firmwareFile             FirmwareFile
	daemonState              []daemonState
	needsRebootAfterFlashing bool
	cleanupTime              time.Duration
	dutTempDir               string
}

// NewFirmwareTest creates and initializes a new fingerprint firmware test.
// enableHWWP indicates whether the test should enable hardware write protect.
// enableSWWP indicates whether the test should enable software write protect.
func NewFirmwareTest(ctx context.Context, d *rpcdut.RPCDUT, servoSpec, outDir string, firmwareFile *FirmwareFile, enableHWWP, enableSWWP bool) (firmwareTest *FirmwareTest, initError error) {
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to servo")
	}

	t := &FirmwareTest{d: d, servo: pxy}
	// Close servo connection when this function is going to return an error.
	defer func() {
		if initError != nil {
			t.servo.Close(ctx)
		}
	}()

	t.fpBoard, err = Board(ctx, t.d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get fingerprint board")
	}

	t.firmwareFile = *firmwareFile
	if firmwareFile.KeyType == KeyTypeMp {
		if err := ValidateBuildFwFile(ctx, t.d, t.fpBoard, firmwareFile.FilePath); err != nil {
			return nil, errors.Wrap(err, "failed to validate MP build firmware file")
		}
	}
	t.needsRebootAfterFlashing, err = NeedsRebootAfterFlashing(ctx, t.d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine if reboot is needed")
	}

	// Disable rootfs verification. It's necessary in order to disable
	// FP updater and biod upstart job if board needs reboot after flashing.
	rootfsIsWritable, err := sysutil.IsRootfsWritable(ctx, t.d.RPC())
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if rootfs is writable")
	}
	if !rootfsIsWritable && t.needsRebootAfterFlashing {
		testing.ContextLog(ctx, "Making rootfs writable")
		// Since MakeRootfsWritable will reboot the device, we must call
		// RPCClose/RPCDial before/after calling MakeRootfsWritable.
		if err := t.d.RPCClose(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to close rpc")
		}
		// Rootfs must be writable in order to disable the upstart job.
		if err := sysutil.MakeRootfsWritable(ctx, t.d.DUT(), t.d.RPCHint()); err != nil {
			return nil, errors.Wrap(err, "failed to make rootfs writable")
		}
		if err := t.d.RPCDial(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to redial rpc")
		}
	} else if rootfsIsWritable && !t.needsRebootAfterFlashing {
		testing.ContextLog(ctx, "WARNING: The rootfs is writable")
	}

	t.daemonState, err = stopDaemons(ctx, t.UpstartService(), []string{
		biodUpstartJobName,
		// TODO(b/183123775): Remove when bug is fixed.
		//  Disabling powerd to prevent the display from turning off, which kills
		//  USB on some platforms.
		powerdUpstartJobName,
	})
	// Start daemons when this function is going to return an error.
	defer func() {
		if initError != nil {
			testing.ContextLog(ctx, "NewFirmwareTest failed, restore daemon state")
			if err := restoreDaemons(ctx, t.UpstartService(), t.daemonState); err != nil {
				testing.ContextLog(ctx, "Failed to restart daemons: ", err)
			}
		}
	}()
	// Check if daemons were stopped correctly.
	if err != nil {
		return nil, err
	}

	t.cleanupTime = timeForCleanup

	if t.needsRebootAfterFlashing || (t.firmwareFile.KeyType != KeyTypeMp) {
		// Disable biod upstart job so that it doesn't interfere with the test when
		// we reboot.
		testing.ContextLogf(ctx, "Disabling %s job", biodUpstartJobName)
		if _, err := t.UpstartService().DisableJob(ctx, &platform.DisableJobRequest{JobName: biodUpstartJobName}); err != nil {
			return nil, errors.Wrap(err, "failed to disable biod upstart job")
		}
		// Enable biod service when this function is going to return an error.
		defer func() {
			if initError != nil {
				testing.ContextLog(ctx, "NewFirmwareTest failed, let's re-enable biod upstart job")
				if _, err := t.UpstartService().EnableJob(ctx, &platform.EnableJobRequest{JobName: biodUpstartJobName}); err != nil {
					testing.ContextLog(ctx, "Failed to re-enable biod upstart job: ", err)
				}
			}
		}()
		// Disable FP updater so that it doesn't interfere with the test when we reboot.
		if err := DisableFPUpdater(ctx, t.d); err != nil {
			return nil, errors.Wrap(err, "failed to disable updater")
		}
		// Enable FP updater when this function is going to return an error.
		defer func() {
			if initError != nil {
				testing.ContextLog(ctx, "NewFirmwareTest failed, let's re-enable FP updater")
				if err := EnableFPUpdater(ctx, d); err != nil {
					testing.ContextLog(ctx, "Failed to re-enable FP updater: ", err)
				}
			}
		}()

		// Account for the additional time that rebooting adds.
		t.cleanupTime += 3 * time.Minute
	}

	t.dutTempDir, err = t.DutfsClient().TempDir(ctx, "", dutTempPathPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create remote working directory")
	}

	// Check FPMCU state and reflash if needed. Remove SWWP if needed.
	if err := InitializeKnownState(ctx, t.d, outDir, pxy, t.fpBoard, t.firmwareFile, t.needsRebootAfterFlashing, !enableSWWP); err != nil {
		return nil, errors.Wrap(err, "initializing known state failed")
	}

	// Double check our work in the previous step.
	if err := CheckValidFlashState(ctx, t.d, t.fpBoard, t.firmwareFile); err != nil {
		return nil, err
	}

	if err := InitializeHWAndSWWriteProtect(ctx, t.d, pxy, t.fpBoard, enableHWWP, enableSWWP); err != nil {
		return nil, errors.Wrap(err, "initializing write protect failed")
	}

	return t, nil
}

// Close cleans up the fingerprint test and restore the FPMCU firmware to the
// original image and state.
func (t *FirmwareTest) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down")

	// Always close servo connection no matter what happens.
	defer t.servo.Close(ctx)

	// The test can fail when DUT is disconnected (e.g. context timeout
	// while reconnecting to DUT). In this case we should attempt to connect
	// to the DUT. When connecting fails, we shouldn't proceed because
	// without healthy connection there is nothing we can do.
	if !t.d.RPCConnected(ctx) {
		testing.ContextLog(ctx, "Reconnecting to the DUT")
		if err := t.d.Connect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
	}

	var firstErr error

	// Always flash MP firmware during clean up.
	firmwareFile, err := NewMPFirmwareFile(ctx, t.d)
	if err != nil {
		firstErr = err
	}
	if err := ReimageFPMCU(ctx, t.d, t.servo, firmwareFile.FilePath, t.needsRebootAfterFlashing); err != nil {
		// ReimageFPMCU reboots the DUT at least once. Sometimes after
		// reboot, the connection to the DUT is broken. In this case
		// we should return error now, because further executing will
		// result in nil pointer dereference, because RPC connection is
		// not available.
		if !t.d.RPCConnected(ctx) {
			return errors.Wrap(err, "lost connection to the DUT")
		}
		firstErr = err
	}

	if t.needsRebootAfterFlashing || (t.firmwareFile.KeyType != KeyTypeMp) {
		// If biod upstart job disabled, re-enable it
		resp, err := t.UpstartService().IsJobEnabled(ctx, &platform.IsJobEnabledRequest{JobName: biodUpstartJobName})
		if err == nil && !resp.Enabled {
			testing.ContextLogf(ctx, "Enabling %s job", biodUpstartJobName)
			if _, err := t.UpstartService().EnableJob(ctx, &platform.EnableJobRequest{JobName: biodUpstartJobName}); err != nil && firstErr == nil {
				firstErr = err
			}
		} else if err != nil && firstErr == nil {
			firstErr = err
		}

		// If FP updater disabled, re-enable it
		fpUpdaterEnabled, err := IsFPUpdaterEnabled(ctx, t.d)
		if err == nil && !fpUpdaterEnabled {
			if err := EnableFPUpdater(ctx, t.d); err != nil && firstErr == nil {
				firstErr = err
			}
		} else if err != nil && firstErr == nil {
			firstErr = err
		}

		// Delete temporary working directory and contents
		// If we rebooted, the directory may no longer exist.
		tempDirExists, err := t.DutfsClient().Exists(ctx, t.dutTempDir)
		if err == nil && tempDirExists {
			if err := t.DutfsClient().RemoveAll(ctx, t.dutTempDir); err != nil && firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to remove temp directory: %q", t.dutTempDir)
			}
		} else if err != nil && firstErr == nil {
			firstErr = errors.Wrapf(err, "failed to check existence of temp directory: %q", t.dutTempDir)
		}
	}

	if err := restoreDaemons(ctx, t.UpstartService(), t.daemonState); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// DUT gets the RPCDUT.
func (t *FirmwareTest) DUT() *rpcdut.RPCDUT {
	return t.d
}

// Servo gets the servo proxy.
func (t *FirmwareTest) Servo() *servo.Proxy {
	return t.servo
}

// RPCClient gets the RPC client.
func (t *FirmwareTest) RPCClient() *rpc.Client {
	return t.d.RPC()
}

// UpstartService gets the upstart service client.
func (t *FirmwareTest) UpstartService() platform.UpstartServiceClient {
	return platform.NewUpstartServiceClient(t.RPCClient().Conn)
}

// FirmwareFile gets the firmware file.
func (t *FirmwareTest) FirmwareFile() FirmwareFile {
	return t.firmwareFile
}

// NeedsRebootAfterFlashing describes whether DUT needs to be rebooted after flashing.
func (t *FirmwareTest) NeedsRebootAfterFlashing() bool {
	return t.needsRebootAfterFlashing
}

// DutfsClient gets the dutfs client.
func (t *FirmwareTest) DutfsClient() *dutfs.Client {
	return dutfs.NewClient(t.RPCClient().Conn)
}

// CleanupTime gets the amount of time needed for cleanup.
func (t *FirmwareTest) CleanupTime() time.Duration {
	return t.cleanupTime
}

// DUTTempDir gets the temporary directory created on the DUT.
func (t *FirmwareTest) DUTTempDir() string {
	return t.dutTempDir
}

// FPBoard gets the fingerprint board name.
func (t *FirmwareTest) FPBoard() fp.BoardName {
	return t.fpBoard
}

type daemonState struct {
	name       string
	wasRunning bool // True if daemon was originally running.
}

// stopDaemons stops the specified daemons and returns their original state.
func stopDaemons(ctx context.Context, upstartService platform.UpstartServiceClient, daemons []string) ([]daemonState, error) {
	var ret []daemonState
	for _, name := range daemons {
		status, err := upstartService.JobStatus(ctx, &platform.JobStatusRequest{JobName: name})
		if err != nil {
			return ret, errors.Wrap(err, "failed to get status for "+name)
		}

		daemonWasRunning := upstart.State(status.GetState()) == upstart.RunningState

		if daemonWasRunning {
			testing.ContextLog(ctx, "Stopping ", name)
			if _, err := upstartService.StopJob(ctx, &platform.StopJobRequest{
				JobName: name,
			}); err != nil {
				return ret, errors.Wrap(err, "failed to stop "+name)
			}
		}

		ret = append(ret, daemonState{
			name:       name,
			wasRunning: daemonWasRunning,
		})
	}

	return ret, nil
}

// restoreDaemons restores the daemons to the state provided in daemonState.
func restoreDaemons(ctx context.Context, upstartService platform.UpstartServiceClient, daemons []daemonState) error {
	var firstErr error

	for i := len(daemons) - 1; i >= 0; i-- {
		daemon := daemons[i]

		testing.ContextLog(ctx, "Checking state for ", daemon.name)
		status, err := upstartService.JobStatus(ctx, &platform.JobStatusRequest{JobName: daemon.name})
		if err != nil {
			testing.ContextLog(ctx, "Failed to get state for "+daemon.name+": ", err)
			if firstErr != nil {
				firstErr = err
			}
			continue
		}

		running := upstart.State(status.GetState()) == upstart.RunningState

		if running != daemon.wasRunning {
			if running {
				testing.ContextLog(ctx, "Stopping ", daemon.name)
				_, err := upstartService.StopJob(ctx, &platform.StopJobRequest{
					JobName: daemon.name,
				})
				if err != nil {
					testing.ContextLog(ctx, "Failed to stop "+daemon.name+": ", err)
					if firstErr != nil {
						firstErr = err
					}
				}
			} else {
				testing.ContextLog(ctx, "Starting ", daemon.name)
				_, err := upstartService.StartJob(ctx, &platform.StartJobRequest{
					JobName: daemon.name,
				})
				if err != nil {
					testing.ContextLog(ctx, "Failed to start "+daemon.name+": ", err)
					if firstErr != nil {
						firstErr = err
					}
				}
			}
		}
	}
	return firstErr
}

// IsFPUpdaterEnabled returns true if the fingerprint updater is enabled.
func IsFPUpdaterEnabled(ctx context.Context, d *rpcdut.RPCDUT) (bool, error) {
	fs := dutfs.NewClient(d.RPC().Conn)
	disabled, err := fs.Exists(ctx, filepath.Join(fp.FirmwareFilePath, disableFpUpdaterFile))
	return !disabled, err
}

// EnableFPUpdater enables the fingerprint updater if it is disabled.
func EnableFPUpdater(ctx context.Context, d *rpcdut.RPCDUT) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	testing.ContextLog(ctx, "Enabling the fingerprint updater")
	disableFpUpdaterPath := filepath.Join(fp.FirmwareFilePath, disableFpUpdaterFile)
	if err := fs.Remove(ctx, disableFpUpdaterPath); err != nil {
		return errors.Wrapf(err, "failed to remove %q", disableFpUpdaterPath)
	}
	// Sync filesystem to make sure that FP updater is enabled correctly.
	if err := d.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to sync DUT")
	}
	return nil
}

// DisableFPUpdater disables the fingerprint updater if it is enabled.
func DisableFPUpdater(ctx context.Context, d *rpcdut.RPCDUT) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	testing.ContextLog(ctx, "Disabling the fingerprint updater")
	disableFpUpdaterPath := filepath.Join(fp.FirmwareFilePath, disableFpUpdaterFile)
	if err := fs.WriteFile(ctx, disableFpUpdaterPath, nil, 0); err != nil {
		return errors.Wrapf(err, "failed to create %q", disableFpUpdaterPath)
	}
	// Sync filesystem to make sure that FP updater is disabled correctly.
	if err := d.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to sync DUT")
	}
	return nil
}
