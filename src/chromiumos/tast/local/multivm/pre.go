// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

var noVMStartedPre = &preImpl{
	"multivm_no_vm",
	NewStateManager(
		ChromeOptions{
			timeout: chrome.LoginTimeout,
		},
		nil,
		nil,
	),
}

// NoVMStarted returns a Precondition that logs into Chrome without starting any
// VMs.
func NoVMStarted() testing.Precondition {
	return noVMStartedPre
}

var arcCrostiniStartedPre = &preImpl{
	"multivm_arc_crostini",
	NewStateManager(
		ChromeOptions{
			timeout: chrome.LoginTimeout,
		},
		&ARCOptions{
			timeout: arc.BootTimeout,
		},
		&CrostiniOptions{
			mode:           cui.Component,
			largeContainer: false,
			debianVersion:  vm.DebianBuster,
			timeout:        7 * time.Minute,
		},
	),
}

// ArcCrostiniStarted returns a Precondition that logs into Chrome and starts
// ARCVM an Crostini.
func ArcCrostiniStarted() testing.Precondition {
	return arcCrostiniStartedPre
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
		nil,
	),
}

// ArcStarted returns a Precondition that logs into Chrome and starts ARCVM.
func ArcStarted() testing.Precondition {
	return arcStartedPre
}

var crostiniStartedPre = &preImpl{
	"multivm_crostini",
	NewStateManager(
		ChromeOptions{
			timeout: chrome.LoginTimeout,
		},
		nil,
		&CrostiniOptions{
			mode:           cui.Component,
			largeContainer: false,
			debianVersion:  vm.DebianBuster,
			timeout:        7 * time.Minute,
		},
	),
}

// CrostiniStarted returns a Precondition that logs into Chrome and starts
// Crostini.
func CrostiniStarted() testing.Precondition {
	return crostiniStartedPre
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
	// Available if useCrostini provided to preImpl
	Crostini *vm.Container
}

func (p *preImpl) String() string { return p.name }
func (p *preImpl) Timeout() time.Duration {
	timeout := p.vmState.crOptions.timeout
	if p.vmState.useARC != nil {
		timeout += p.vmState.useARC.timeout
	}
	if p.vmState.useCrostini != nil {
		timeout += p.vmState.useCrostini.timeout
	}
	return timeout
}

func (p *preImpl) preData() *PreData {
	return &PreData{
		Chrome:      p.vmState.Chrome(),
		TestAPIConn: p.vmState.TestAPIConn(),
		Keyboard:    p.vmState.Keyboard(),
		ARC:         p.vmState.ARC(),
		Crostini:    p.vmState.Crostini(),
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
