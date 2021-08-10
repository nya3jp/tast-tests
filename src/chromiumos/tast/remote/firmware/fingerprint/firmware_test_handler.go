// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/upstart"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/remote/sysutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

// FirmwareTest provides a common framework for fingerprint firmware tests.
type FirmwareTest struct {
	d                        *dut.DUT
	servo                    *servo.Proxy
	cl                       *rpc.Client
	rpcHint                  *testing.RPCHint
	fpBoard                  FPBoardName
	buildFwFile              string
	upstartService           platform.UpstartServiceClient
	daemonState              []daemonState
	needsRebootAfterFlashing bool
	dutfsClient              *dutfs.Client
	cleanupTime              time.Duration
	dutTempDir               string
}

// NewFirmwareTest creates and initializes a new fingerprint firmware test.
// enableHWWP indicates whether the test should enable hardware write protect.
// enableSWWP indicates whether the test should enable software write protect.
func NewFirmwareTest(ctx context.Context, d *dut.DUT, servoSpec string, hint *testing.RPCHint, outDir string, enableHWWP, enableSWWP bool) (*FirmwareTest, error) {
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to servo")
	}

	cl, err := rpc.Dial(ctx, d, hint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}

	upstartService := platform.NewUpstartServiceClient(cl.Conn)

	daemonState, err := stopDaemons(ctx, upstartService, []string{
		biodUpstartJobName,
		// TODO(b/183123775): Remove when bug is fixed.
		//  Disabling powerd to prevent the display from turning off, which kills
		//  USB on some platforms.
		powerdUpstartJobName,
	})
	if err != nil {
		return nil, err
	}

	fpBoard, err := Board(ctx, d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get fingerprint board")
	}

	buildFwFile, err := FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get build firmware file path")
	}

	dutfsClient := dutfs.NewClient(cl.Conn)

	if err := ValidateBuildFwFile(ctx, d, dutfsClient, fpBoard, buildFwFile); err != nil {
		return nil, errors.Wrap(err, "failed to validate build firmware file")
	}

	needsReboot, err := NeedsRebootAfterFlashing(ctx, d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine if reboot is needed")
	}

	cleanupTime := timeForCleanup

	if needsReboot {
		// Rootfs must be writable in order to disable the upstart job
		if err := sysutil.MakeRootfsWritable(ctx, d, hint); err != nil {
			return nil, errors.Wrap(err, "failed to make rootfs writable")
		}

		// disable biod upstart job so that it doesn't interfere with the test when
		// we reboot.
		if _, err := upstartService.DisableJob(ctx, &platform.DisableJobRequest{JobName: biodUpstartJobName}); err != nil {
			return nil, errors.Wrap(err, "failed to disable biod upstart job")
		}

		// disable FP updater so that it doesn't interfere with the test when we reboot.
		if err := disableFPUpdater(ctx, dutfsClient); err != nil {
			return nil, errors.Wrap(err, "failed to disable updater")
		}

		// Account for the additional time that rebooting adds.
		cleanupTime += 3 * time.Minute
	} else {
		// If we're not on a device that needs to be rebooted, the rootfs should not
		// be writable.
		rootfsIsWritable, err := sysutil.IsRootfsWritable(ctx, cl)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if rootfs is writable")
		}
		if rootfsIsWritable {
			testing.ContextLog(ctx, "WARNING: The rootfs is writable")
		}
	}

	dutTempDir, err := dutfsClient.TempDir(ctx, "", dutTempPathPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create remote working directory")
	}

	if err := InitializeKnownState(ctx, d, dutfsClient, outDir, pxy, fpBoard, buildFwFile, needsReboot); err != nil {
		return nil, errors.Wrap(err, "initializing known state failed")
	}

	if err := CheckInitialState(ctx, d, dutfsClient, fpBoard, buildFwFile); err != nil {
		return nil, err
	}

	if err := InitializeHWAndSWWriteProtect(ctx, d, pxy, fpBoard, enableHWWP, enableSWWP); err != nil {
		return nil, errors.Wrap(err, "initializing write protect failed")
	}

	return &FirmwareTest{
			d:                        d,
			servo:                    pxy,
			cl:                       cl,
			rpcHint:                  hint,
			fpBoard:                  fpBoard,
			buildFwFile:              buildFwFile,
			upstartService:           upstartService,
			daemonState:              daemonState,
			needsRebootAfterFlashing: needsReboot,
			dutfsClient:              dutfsClient,
			cleanupTime:              cleanupTime,
			dutTempDir:               dutTempDir,
		},
		nil
}

// Close cleans up the fingerprint test and restore the FPMCU firmware to the
// original image and state.
func (t *FirmwareTest) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down")
	var firstErr error

	if err := ReimageFPMCU(ctx, t.d, t.servo, t.needsRebootAfterFlashing); err != nil {
		firstErr = err
	}

	// TODO(https://crbug.com/1195936): ReimageFPMCU reboots, which causes gRPC
	//  to lose its connection.
	cl, err := rpc.Dial(ctx, t.d, t.rpcHint, "cros")
	if err != nil && firstErr == nil {
		firstErr = err
	}

	if cl != nil {
		t.cl = cl
		t.upstartService = platform.NewUpstartServiceClient(cl.Conn)
		t.dutfsClient = dutfs.NewClient(cl.Conn)

		if t.needsRebootAfterFlashing {
			// If biod upstart job disabled, re-enable it
			resp, err := t.upstartService.IsJobEnabled(ctx, &platform.IsJobEnabledRequest{JobName: biodUpstartJobName})
			if err == nil && !resp.Enabled {
				if _, err := t.upstartService.EnableJob(ctx, &platform.EnableJobRequest{JobName: biodUpstartJobName}); err != nil && firstErr == nil {
					firstErr = err
				}
			} else if err != nil && firstErr == nil {
				firstErr = err
			}

			// If FP updater disabled, re-enable it
			fpUpdaterEnabled, err := isFPUpdaterEnabled(ctx, t.dutfsClient)
			if err == nil && !fpUpdaterEnabled {
				if err := enableFPUpdater(ctx, t.dutfsClient); err != nil && firstErr == nil {
					firstErr = err
				}
			} else if err != nil && firstErr == nil {
				firstErr = err
			}

			// Delete temporary working directory and contents
			// If we rebooted, the directory may no longer exist.
			tempDirExists, err := t.dutfsClient.Exists(ctx, t.dutTempDir)
			if err == nil && tempDirExists {
				if err := t.dutfsClient.RemoveAll(ctx, t.dutTempDir); err != nil && firstErr == nil {
					firstErr = errors.Wrapf(err, "failed to remove temp directory: %q", t.dutTempDir)
				}
			} else if err != nil && firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to check existence of temp directory: %q", t.dutTempDir)
			}
		}

		if err := restoreDaemons(ctx, t.upstartService, t.daemonState); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	t.servo.Close(ctx)

	if err := t.cl.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// DUT gets the DUT.
func (t *FirmwareTest) DUT() *dut.DUT {
	return t.d
}

// Servo gets the servo proxy.
func (t *FirmwareTest) Servo() *servo.Proxy {
	return t.servo
}

// RPCClient gets the RPC client.
func (t *FirmwareTest) RPCClient() *rpc.Client {
	return t.cl
}

// BuildFwFile gets the firmware file.
func (t *FirmwareTest) BuildFwFile() string {
	return t.buildFwFile
}

// NeedsRebootAfterFlashing describes whether DUT needs to be rebooted after flashing.
func (t *FirmwareTest) NeedsRebootAfterFlashing() bool {
	return t.needsRebootAfterFlashing
}

// DutfsClient gets the dutfs client.
func (t *FirmwareTest) DutfsClient() *dutfs.Client {
	return t.dutfsClient
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
func (t *FirmwareTest) FPBoard() FPBoardName {
	return t.fpBoard
}

type daemonState struct {
	name       string
	wasRunning bool // true if daemon was originally running
}

// stopDaemons stops the specified daemons and returns their original state.
func stopDaemons(ctx context.Context, upstartService platform.UpstartServiceClient, daemons []string) ([]daemonState, error) {
	var ret []daemonState
	for _, name := range daemons {
		status, err := upstartService.JobStatus(ctx, &platform.JobStatusRequest{JobName: name})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get status for"+name)
		}

		daemonWasRunning := upstart.State(status.GetState()) == upstart.RunningState

		if daemonWasRunning {
			testing.ContextLog(ctx, "Stopping ", name)
			_, err := upstartService.StopJob(ctx, &platform.StopJobRequest{
				JobName: name,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to stop "+name)
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
			testing.ContextLog(ctx, "Failed to get state for "+daemon.name)
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
					testing.ContextLog(ctx, "Failed to stop "+daemon.name)
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
					testing.ContextLog(ctx, "Failed to start "+daemon.name)
					if firstErr != nil {
						firstErr = err
					}
				}
			}
		}
	}
	return firstErr
}

// isFPUpdaterEnabled returns true if the fingerprint updater is enabled.
func isFPUpdaterEnabled(ctx context.Context, fs *dutfs.Client) (bool, error) {
	return fs.Exists(ctx, filepath.Join(fingerprintFirmwarePathBase, disableFpUpdaterFile))
}

// enableFPUpdater enables the fingerprint updater if it is disabled.
func enableFPUpdater(ctx context.Context, fs *dutfs.Client) error {
	testing.ContextLog(ctx, "Enabling the fingerprint updater")
	return fs.Remove(ctx, filepath.Join(fingerprintFirmwarePathBase, disableFpUpdaterFile))
}

// disableFPUpdater disables the fingerprint updater if it is enabled.
func disableFPUpdater(ctx context.Context, fs *dutfs.Client) error {
	testing.ContextLog(ctx, "Disabling the fingerprint updater")
	return fs.WriteFile(ctx, filepath.Join(fingerprintFirmwarePathBase, disableFpUpdaterFile), nil, 0)
}
