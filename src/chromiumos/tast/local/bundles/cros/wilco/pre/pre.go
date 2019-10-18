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

// SystemState describes the desired system state
type SystemState struct {
	// WilcoDtcDaemonRunning ensures the Wilco DTC Support Daemon is running
	WilcoDtcDaemonRunning bool
	// WilcoDtcDaemonRunning ensures the Wilco DTC VM is running
	WilcoDtcVMRunning bool
	// VMConfig is the configuration for the DTC VM
	VMConfig wilco.VMConfig
	// DDVDBusAvailable ensures the DDVDBus is available when the test starts
	DDVDBusAvailable bool
}

type preImpl struct {
	name  string
	state SystemState
	setUp bool
}

// NewPrecondition creates a new precondition that can be shared by tests
// that reqire a combination of the Wilco DTC Daemon and VM to be running
func NewPrecondition(suffix string, state SystemState) *preImpl {
	return &preImpl{
		name:  "wilco_dtc_" + suffix,
		state: state,
		setUp: false,
	}
}

// WilcoDtcSuportdAPI Precondition sets up the system to test the API provided
// in wilco_dtc_supportd.proto
var WilcoDtcSuportdAPI = NewPrecondition("suportd_api", SystemState{
	WilcoDtcDaemonRunning: true,
	WilcoDtcVMRunning:     true,
	VMConfig: wilco.VMConfig{
		StartProcesses: false,
		TestDBusConfig: false,
	},
	DDVDBusAvailable: false,
})

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return 5 * time.Second }

func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.setUp {
		return &p.state
	}

	ctx, st := timing.Start(ctx, "setup_wilco_dtc")
	defer st.End()

	if p.state.WilcoDtcVMRunning {
		if err := wilco.StartVM(ctx, &p.state.VMConfig); err != nil {
			s.Fatal("Unable to start the Wilco DTC VM: ", err)
		}
	}

	if p.state.WilcoDtcDaemonRunning {
		if err := wilco.StartSupportd(ctx); err != nil {
			s.Fatal("Unable to start the Wilco DTC Support Daemon: ", err)
		}
	}

	if p.state.DDVDBusAvailable {
		if err := wilco.WaitForDDVDBus(ctx); err != nil {
			s.Fatal("DDV dbus service is not available: ", err)
		}
	}

	p.setUp = true

	return &p.state
}

func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "shutdown_wilco_dtc")
	defer st.End()

	if p.state.WilcoDtcDaemonRunning {
		if err := wilco.StopSupportd(ctx); err != nil {
			s.Fatal("Unable to stop the Wilco DTC Support Daemon: ", err)
		}
	}

	if p.state.WilcoDtcVMRunning {
		if err := wilco.StopVM(ctx); err != nil {
			s.Fatal("Unable to stop the Wilco DTC VM: ", err)
		}
	}
}
