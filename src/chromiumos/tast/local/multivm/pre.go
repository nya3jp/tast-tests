// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// DefaultChromeOptions defines the default options for creating Chrome.
var DefaultChromeOptions = ChromeOptions{
	Timeout:     chrome.LoginTimeout,
	BrowserType: browser.TypeAsh,
}

// LacrosChromeOptions creates Lacros Chrome.
var LacrosChromeOptions = ChromeOptions{
	Timeout:     chrome.LoginTimeout,
	BrowserType: browser.TypeLacros,
}

// DefaultARCOptions defines the default options for starting ARC VM.
var DefaultARCOptions = ARCOptions{}

// DefaultCrostiniOptions defines the default options for starting Crostini.
var DefaultCrostiniOptions = CrostiniOptions{
	LargeContainer: false,
	DebianVersion:  vm.DebianBuster,
}

var noVMStartedPre = NewMultiVMPrecondition(
	"multivm_no_vm",
	NewStateManager(
		DefaultChromeOptions,
	))

// NoVMStarted returns a Precondition that logs into Chrome without starting any
// VMs.
func NoVMStarted() testing.Precondition {
	return noVMStartedPre
}

var noVMLacrosStartedPre = NewMultiVMPrecondition(
	"multivm_no_vm_lacros",
	NewStateManager(
		LacrosChromeOptions,
	))

// NoVMLacrosStarted returns a Precondition that logs into Lacros Chrome without
// starting any VMs.
func NoVMLacrosStarted() testing.Precondition {
	return noVMLacrosStartedPre
}

var arcCrostiniStartedPre = NewMultiVMPrecondition(
	"multivm_arc_crostini",
	NewStateManager(
		DefaultChromeOptions,
		DefaultARCOptions,
		DefaultCrostiniOptions,
	))

// ArcCrostiniStarted returns a Precondition that logs into Chrome and starts
// ARCVM an Crostini.
func ArcCrostiniStarted() testing.Precondition {
	return arcCrostiniStartedPre
}

var arcCrostiniLacrosStartedPre = NewMultiVMPrecondition(
	"multivm_arc_crostini_lacros",
	NewStateManager(
		LacrosChromeOptions,
		DefaultARCOptions,
		DefaultCrostiniOptions,
	))

// ArcCrostiniLacrosStarted returns a Precondition that logs into Lacros Chrome
// and starts ARCVM an Crostini.
func ArcCrostiniLacrosStarted() testing.Precondition {
	return arcCrostiniLacrosStartedPre
}

var arcStartedPre = NewMultiVMPrecondition(
	"multivm_arc",
	NewStateManager(
		DefaultChromeOptions,
		DefaultARCOptions,
	))

// ArcStarted returns a Precondition that logs into Chrome and starts ARCVM.
func ArcStarted() testing.Precondition {
	return arcStartedPre
}

var arcLacrosStartedPre = NewMultiVMPrecondition(
	"multivm_arc_lacros",
	NewStateManager(
		LacrosChromeOptions,
		DefaultARCOptions,
	))

// ArcLacrosStarted returns a Precondition that logs into Lacros Chrome and
// starts ARCVM.
func ArcLacrosStarted() testing.Precondition {
	return arcLacrosStartedPre
}

var crostiniStartedPre = NewMultiVMPrecondition(
	"multivm_crostini",
	NewStateManager(
		DefaultChromeOptions,
		DefaultCrostiniOptions,
	))

// CrostiniStarted returns a Precondition that logs into Chrome and starts
// Crostini.
func CrostiniStarted() testing.Precondition {
	return crostiniStartedPre
}

var crostiniLacrosStartedPre = NewMultiVMPrecondition(
	"multivm_crostini_lacros",
	NewStateManager(
		LacrosChromeOptions,
		DefaultCrostiniOptions,
	))

// CrostiniLacrosStarted returns a Precondition that logs into Lacros Chrome and
// starts Crostini.
func CrostiniLacrosStarted() testing.Precondition {
	return crostiniLacrosStartedPre
}

var arcCrostiniStartedWithDNSProxyPre = NewMultiVMPrecondition(
	"multivm_arc_crostini_dns_proxy",
	NewStateManager(
		ChromeOptions{EnableFeatures: []string{"EnableDnsProxy", "DnsProxyEnableDOH"}, Timeout: chrome.LoginTimeout},
		DefaultARCOptions,
		DefaultCrostiniOptions,
	))

// ArcCrostiniStartedWithDNSProxy returns a Precondition that logs into Chrome with DNS proxy
// enabled and starts ARC and Crostini.
func ArcCrostiniStartedWithDNSProxy() testing.Precondition {
	return arcCrostiniStartedWithDNSProxyPre
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
	// The VMs set up by the precondition. It is recommended to access via the
	// VM-defined helper methods, such as multivm.ARCFromPre and
	// multivm.CrostiniFromPre.
	VMs map[string]interface{}
}

// NewMultiVMPrecondition returns a new precondition that can be used be
// used by tests that expect multiple VMs to be started at the start of the
// test.
func NewMultiVMPrecondition(name string, vmState StateManager) testing.Precondition {
	return &preImpl{
		name:    name,
		vmState: vmState,
	}
}

func (p *preImpl) String() string { return p.name }
func (p *preImpl) Timeout() time.Duration {
	return p.vmState.Timeout()
}

func (p *preImpl) preData() *PreData {
	return &PreData{
		Chrome:      p.vmState.Chrome(),
		TestAPIConn: p.vmState.TestAPIConn(),
		Keyboard:    p.vmState.Keyboard(),
		VMs:         p.vmState.VMs(),
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
