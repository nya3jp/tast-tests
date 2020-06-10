// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"time"

	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// SystemState describes the desired system state.
type SystemState struct {
	// WilcoDTCDaemonRunning ensures the Wilco DTC Support Daemon is running.
	WilcoDTCDaemonRunning bool
	// WilcoDTCVMRunning ensures the Wilco DTC VM is running.
	WilcoDTCVMRunning bool
	// VMConfig is the configuration for the DTC VM.
	VMConfig wilco.VMConfig
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name                string
	state               SystemState
	setUp               bool
	wilcoDTCSupportdPID int
	wilcoDTCVMPID       int
}

// NewPrecondition creates a new precondition that can be shared by tests
// that reqire a combination of the Wilco DTC Daemon and VM to be running.
func NewPrecondition(suffix string, state SystemState) *preImpl {
	return &preImpl{
		name:  "wilco_dtc_" + suffix,
		state: state,
		setUp: false,
	}
}

// WilcoDtcSupportdAPI Precondition sets up the system to test the API provided
// in wilco_dtc_supportd.proto.
var WilcoDtcSupportdAPI = NewPrecondition("supportd_api", SystemState{
	WilcoDTCDaemonRunning: true,
	WilcoDTCVMRunning:     true,
	VMConfig: wilco.VMConfig{
		StartProcesses: false,
		TestDBusConfig: false,
	},
})

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return 15 * time.Second }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing the current state that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	// We assume that tests do not interfere with the Wilco DTC VM and Daemon.
	if p.setUp {
		if p.state.WilcoDTCDaemonRunning {
			pid, err := wilco.SupportdPID(ctx)

			if err != nil {
				s.Fatal("Unable to get Wilco DTC Support Daemon PID: ", err)
			}

			if p.wilcoDTCSupportdPID != pid {
				s.Error("The Wilco DTC Support Daemon PID changed while testing")
			}

			// Restart the Wilco DTC Daemon to flush the queued events.
			if err := wilco.StartSupportd(ctx); err != nil {
				s.Fatal("Unable to restart the Wilco DTC Support Daemon: ", err)
			}

			pid, err = wilco.SupportdPID(ctx)
			if err != nil {
				s.Fatal("Unable to get Wilco DTC Support Daemon PID: ", err)
			}

			p.wilcoDTCSupportdPID = pid
		}

		if p.state.WilcoDTCVMRunning {
			pid, err := wilco.VMPID(ctx)

			if err != nil {
				s.Fatal("Unable to get Wilco DTC VM PID: ", err)
			}

			if p.wilcoDTCVMPID != pid {
				s.Error("The Wilco DTC VM PID changed while testing")
			}
		}

		return p.state
	}

	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.state.WilcoDTCVMRunning {
		if err := wilco.StartVM(ctx, &p.state.VMConfig); err != nil {
			s.Fatal("Unable to start the Wilco DTC VM: ", err)
		}

		pid, err := wilco.VMPID(ctx)

		if err != nil {
			s.Fatal("Unable to get Wilco DTC VM PID: ", err)
		}

		p.wilcoDTCVMPID = pid

		if p.state.VMConfig.StartProcesses {
			if err := wilco.WaitForDDVDBus(ctx); err != nil {
				s.Fatal("DDV dbus service is not available: ", err)
			}
		}
	}

	if p.state.WilcoDTCDaemonRunning {
		if err := wilco.StartSupportd(ctx); err != nil {
			s.Fatal("Unable to start the Wilco DTC Support Daemon: ", err)
		}

		pid, err := wilco.SupportdPID(ctx)

		if err != nil {
			s.Fatal("Unable to get Wilco DTC Support Daemon PID: ", err)
		}

		p.wilcoDTCSupportdPID = pid
	}

	p.setUp = true

	return p.state
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	if !p.setUp {
		return
	}

	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	if p.state.WilcoDTCDaemonRunning {
		pid, err := wilco.SupportdPID(ctx)

		if err != nil {
			s.Fatal("Unable to get Wilco DTC Support Daemon PID: ", err)
		}

		if p.wilcoDTCSupportdPID != pid {
			s.Error("The Wilco DTC Support Daemon PID changed while testing")
		}

		if err := wilco.StopSupportd(ctx); err != nil {
			s.Error("Unable to stop the Wilco DTC Support Daemon: ", err)
		}
	}

	if p.state.WilcoDTCVMRunning {
		pid, err := wilco.VMPID(ctx)

		if err != nil {
			s.Fatal("Unable to get Wilco DTC VM PID: ", err)
		}

		if p.wilcoDTCVMPID != pid {
			s.Error("The Wilco DTC VM PID changed while testing")
		}

		if err := wilco.StopVM(ctx); err != nil {
			s.Error("Unable to stop the Wilco DTC VM: ", err)
		}
	}

	p.setUp = false
}
