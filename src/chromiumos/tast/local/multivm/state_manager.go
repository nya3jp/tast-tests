// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ChromeOptions describe how to run chrome.New.
type ChromeOptions struct {
	EnableFeatures []string // extra Chrome features to enable
	ExtraArgs      []string // passed to Chrome on initialization
	Timeout        time.Duration
	BrowserType    browser.Type
}

// VMOptions describes how to start a VM.
type VMOptions interface {
	// Name returns a stable name for the VM. This is the same name that
	// must be passed to StateManager.VM() to retrieve the VM instance.
	Name() string
	// ChromeOpts returns the Chrome option(s) that should be passed to
	// chrome.New().
	ChromeOpts() []chrome.Option
	// ActivateTimeout returns the time needed to activate the VM.
	ActivateTimeout() time.Duration
	// Activate activates the requested VM. The operation should either
	// succeed completely, or roll the VM back to a deactivated state.
	Activate(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, st StateManagerTestingState) (VMActivation, error)
}

// VMActivation represents an active VM instance. This interface is returned by
// VMOptions.Activate(...) to ensure the methods can only be called after
// activation has occurred.
type VMActivation interface {
	// CheckAndReset checks and cleans a VM, so it can be re-used for another test.
	CheckAndReset(ctx context.Context, st StateManagerTestingState) error
	// Deactivate the VM.
	Deactivate(ctx context.Context) error
	// VM returns a VM-specific object representing the active VM.
	VM() interface{}
}

// StateManager allows Chrome and VMs to be activated, checked and cleaned
// between tests, and deactivated.
type StateManager struct {
	// Activate options.
	crOptions ChromeOptions
	vmOptions []VMOptions

	// Managed Chrome and VMs. Default zero values set inactive state.
	cr       *chrome.Chrome
	vms      map[string]VMActivation
	tconn    *chrome.TestConn
	keyboard *input.KeyboardEventWriter

	active bool
}

// NewStateManager creates a state manager from ChromeOptions, and optional
// VMOptions, depending on the VMs to be launched.
func NewStateManager(crOptions ChromeOptions, vms ...VMOptions) StateManager {
	return StateManager{
		crOptions: crOptions,
		vmOptions: vms,
		vms:       make(map[string]VMActivation),
		cr:        nil,
		tconn:     nil,
		keyboard:  nil,
		active:    false,
	}
}

// StateManagerTestingState is the subset of testing.State or testing.PreState
// needed by StateManager.
type StateManagerTestingState interface {
	DataPath(p string) string
	OutDir() string
	RequiredVar(name string) string
	Var(name string) (val string, ok bool)
	SoftwareDeps() []string
}

// Chrome gets the active chrome.Chrome.
func (s *StateManager) Chrome() *chrome.Chrome {
	if !s.active {
		panic("Do not call Chrome when multivm.StateManager is not active")
	}
	return s.cr
}

// TestAPIConn gets the active chrome.TestConn.
func (s *StateManager) TestAPIConn() *chrome.TestConn {
	if !s.active {
		panic("Do not call TestAPIConn when multivm.StateManager is not active")
	}
	return s.tconn
}

// Keyboard gets the active KeyboardEventWriter.
func (s *StateManager) Keyboard() *input.KeyboardEventWriter {
	if !s.active {
		panic("Do not call Keyboard when multivm.StateManager is not active")
	}
	return s.keyboard
}

// VMs returns the active VMs as map, keyed by a VM-defined name. Test code
// will typically not interact directly with this untyped collection, but use
// VM-specific helper methods like multivm.ARCFromPre, multivm.CrostiniFromPre
// to access it.
func (s *StateManager) VMs() map[string]interface{} {
	if !s.active {
		panic("Do not call VMs when multivm.StateManager is not active")
	}
	result := make(map[string]interface{})
	for k, v := range s.vms {
		result[k] = v.VM()
	}
	return result
}

// Active is true if Chrome and VMs are currently active.
func (s *StateManager) Active() bool {
	return s.active
}

// Timeout returns the total timeout needed to activate Chrome and all VMs.
func (s *StateManager) Timeout() time.Duration {
	duration := s.crOptions.Timeout
	for _, v := range s.vmOptions {
		duration += v.ActivateTimeout()
	}
	return duration
}

// Activate Chrome and any requested VMs.
func (s *StateManager) Activate(ctx context.Context, st StateManagerTestingState) (errRet error) {
	if s.active {
		return errors.New("already active")
	}
	ctx, stage := timing.Start(ctx, "multivm_state_activate")
	defer stage.End()

	defer func() {
		if errRet != nil {
			if err := s.Deactivate(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to Deactivate after failed Activate: ", err)
			}
		}
	}()

	// Chrome.
	if err := func() error {
		ctx, cancel := context.WithTimeout(ctx, s.crOptions.Timeout)
		defer cancel()

		var opts []chrome.Option
		opts = append(opts, chrome.EnableFeatures(s.crOptions.EnableFeatures...), chrome.ExtraArgs(s.crOptions.ExtraArgs...))
		for _, v := range s.vmOptions {
			opts = append(opts, v.ChromeOpts()...)
		}

		testing.ContextLog(ctx, "Creating Chrome")
		var err error
		s.cr, err = browserfixt.NewChrome(ctx, s.crOptions.BrowserType, lacrosfixt.NewConfig(), opts...)
		if err != nil {
			return errors.Wrap(err, "failed to create Chrome")
		}
		chrome.Lock()
		if s.tconn, err = s.cr.TestAPIConn(ctx); err != nil {
			return errors.Wrap(err, "failed to create test API connection")
		}
		if s.keyboard, err = input.Keyboard(ctx); err != nil {
			return errors.Wrap(err, "failed to create keyboard device")
		}
		return nil
	}(); err != nil {
		return err
	}

	for _, v := range s.vmOptions {
		if err := func() error {
			if _, ok := s.vms[v.Name()]; ok {
				return errors.Errorf("a VM with the name %q is already active", v.Name())
			}
			ctx, cancel := context.WithTimeout(ctx, v.ActivateTimeout())
			defer cancel()

			vm, err := v.Activate(ctx, s.cr, s.tconn, st)
			if err != nil {
				return err
			}
			s.vms[v.Name()] = vm
			return nil
		}(); err != nil {
			return errors.Wrapf(err, "failed activating %s", v.Name())
		}
	}

	s.active = true
	return nil
}

// CheckAndReset Chrome and any requested VMs, so they can be re-used for
// another test.
func (s *StateManager) CheckAndReset(ctx context.Context, st StateManagerTestingState) error {
	if !s.active {
		panic("Do not call CheckAndReset when multivm.StateManager is not active")
	}

	ctx, stage := timing.Start(ctx, "multivm_state_check_and_reset")
	defer stage.End()

	// Chrome.
	if err := s.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "failed checking Chrome")
	}
	if err := s.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting Chrome")
	}

	for k, v := range s.vms {
		if err := v.CheckAndReset(ctx, st); err != nil {
			return errors.Wrapf(err, "failed checking and resetting %s", k)
		}
	}

	return nil
}

// Deactivate the state. Safe to call even if not active, or partially active
// because initialization failed.
func (s *StateManager) Deactivate(ctx context.Context) (errRet error) {
	ctx, stage := timing.Start(ctx, "multivm_state_deactivate")
	defer stage.End()

	// NB: We deactivate in reverse order using defer so that if any one part
	// panics, the rest will still complete.

	// NB: Keep this first, so that it runs last.
	defer func() {
		if errRet == nil {
			// We are no longer active if nothing has failed.
			s.active = false
		}
	}()

	if s.cr != nil {
		defer func() {
			chrome.Unlock()
			if err := s.cr.Close(ctx); err != nil {
				if errRet == nil {
					errRet = errors.Wrap(err, "failed to deactivate Chrome")
				} else {
					testing.ContextLog(ctx, "Failed to deactivate Chrome: ", err)
				}
			}
			s.cr = nil
		}()
	}

	if s.keyboard != nil {
		defer func() {
			if err := s.keyboard.Close(); err != nil {
				if errRet == nil {
					errRet = errors.Wrap(err, "failed to deactivate keyboard")
				} else {
					testing.ContextLog(ctx, "Failed to deactivate keyboard: ", err)
				}
			}
			s.keyboard = nil
		}()
	}

	for k, v := range s.vms {
		defer func(k string, v VMActivation) {
			if err := v.Deactivate(ctx); err != nil {
				if errRet == nil {
					errRet = errors.Wrapf(err, "failed to deactivate %s", k)
				} else {
					testing.ContextLogf(ctx, "Failed to deactivate %s: %v", k, err)
				}
			}
			delete(s.vms, k)
		}(k, v)
	}

	return nil
}
