// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

var noVMStartedPre = &preImpl{
	"multivm_no_vm",
	NewStateManager(
		ChromeOptions{
			timeout: chrome.LoginTimeout,
		},
		nil,
	),
}

// NoVMStarted returns a Precondition that logs into Chrome without starting any
// VMs.
func NoVMStarted() testing.Precondition {
	return noVMStartedPre
}

var arcStartedPre = &preImpl{
	"multivm_arc",
	NewStateManager(
		ChromeOptions{
			timeout: chrome.LoginTimeout,
		},
		&ARCOptions{
			timeout: arc.BootTimeout,
		},
	),
}

// ArcStarted returns a Precondition that logs into Chrome and starts ARCVM.
func ArcStarted() testing.Precondition {
	return arcStartedPre
}

type preImpl struct {
	// Configuration.
	name    string // testing.Precondition.String
	vmState StateManager
}

// PreData holds data allowing tests to interact with the VMs requested by their
// precondition.
type PreData struct {
	// Always available.
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	Keyboard    *input.KeyboardEventWriter
	// Available if useARC provided to preImpl.
	ARC *arc.ARC
}

func (p *preImpl) String() string { return p.name }
func (p *preImpl) Timeout() time.Duration {
	timeout := p.vmState.crOptions.timeout
	if p.vmState.useARC != nil {
		timeout += p.vmState.useARC.timeout
	}
	return timeout
}

func (p *preImpl) preData() *PreData {
	return &PreData{
		Chrome:      p.vmState.Chrome(),
		TestAPIConn: p.vmState.TestAPIConn(),
		Keyboard:    p.vmState.Keyboard(),
		ARC:         p.vmState.ARC(),
	}
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if p.vmState.Active() {
		err := p.vmState.CheckAndReset(ctx, s)
		if err == nil {
			return p.preData()
		}
		s.Log("Failed checking or resetting Chrome+VMs: ", err)
		if err := p.vmState.Deactivate(ctx); err != nil {
			s.Fatal("Failed to deactivate Chrome+VMs after check failed: ", err)
		}
	}
	if err := p.vmState.Activate(ctx, s); err != nil {
		s.Fatal("Failed to activate Chrome+VMs: ", err)
	}
	return p.preData()
}

// Close is called after all tests involving this precondition have been run.
// Stops all requested VMs and Chrome.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	if err := p.vmState.Deactivate(ctx); err != nil {
		s.Fatal("Failed to deacivate Chrome+VMs: ", err)
	}
}
