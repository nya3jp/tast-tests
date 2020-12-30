// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ChromeOptions describe how to run chrome.New.
type ChromeOptions struct {
	extraArgs []string // passed to Chrome on initialization
	timeout   time.Duration
}

// ARCOptions describe how to start ARC. Pass nil if ARC is not needed.
type ARCOptions struct {
	timeout time.Duration
}

// CrostiniOptions describe how to start Crostini. Pass nil if Crostini is not
// needed.
type CrostiniOptions struct {
	mode           string                    // Where (download/build artifact) the container image comes from.
	debianVersion  vm.ContainerDebianVersion // OS version of the container image.
	minDiskSize    uint64                    // The minimum size of the VM image in bytes. 0 to use default disk size.
	largeContainer bool
	timeout        time.Duration
}

// StateManager allows Chrome and VMs to be activated, checked and cleaned
// between tests, and deactivated.
type StateManager struct {
	// Activate options.
	crOptions   ChromeOptions
	useARC      *ARCOptions
	useCrostini *CrostiniOptions

	// Managed Chrome and VM. Default zero values set inactive state.
	cr          *chrome.Chrome
	tconn       *chrome.TestConn
	keyboard    *input.KeyboardEventWriter
	arc         *arc.ARC
	arcSnapshot *arc.Snapshot
	crostini    *vm.Container

	active bool
}

// NewStateManager creates a state manager from ChromeOptions, and optional
// ARCOptions and CrostiniOptions, if ARC and/or Crostini are to be launched.
func NewStateManager(crOptions ChromeOptions, useARC *ARCOptions, useCrostini *CrostiniOptions) StateManager {
	return StateManager{
		crOptions:   crOptions,
		useARC:      useARC,
		useCrostini: useCrostini,
		cr:          nil,
		tconn:       nil,
		keyboard:    nil,
		arc:         nil,
		arcSnapshot: nil,
		crostini:    nil,
		active:      false,
	}
}

// StateManagerTestingState is the subset of testing.State or testing.PreState
// needed by StateManager.
type StateManagerTestingState interface {
	DataPath(p string) string
	OutDir() string
	RequiredVar(name string) string
	SoftwareDeps() []string
	Fatal(...interface{})
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

// ARC gets the active arc.ARC, or nil if not active.
func (s *StateManager) ARC() *arc.ARC {
	if !s.active {
		panic("Do not call ARC when multivm.StateManager is not active")
	}
	return s.arc
}

// Crostini gets the active Crostini container, or nil if not active.
func (s *StateManager) Crostini() *vm.Container {
	if !s.active {
		panic("Do not call Crostini when multivm.StateManager is not active")
	}
	return s.crostini
}

// Active is true if Chrome and VMs are currently active.
func (s *StateManager) Active() bool {
	return s.active
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
		ctx, cancel := context.WithTimeout(ctx, s.crOptions.timeout)
		defer cancel()

		var opts []chrome.Option
		if s.useARC != nil {
			opts = append(opts, chrome.ARCEnabled())
		} else {
			opts = append(opts, chrome.ARCDisabled())
		}
		if s.useCrostini != nil {
			opts = append(opts, chrome.ExtraArgs("--vmodule=crostini*=1"))
		}
		opts = append(opts, chrome.ExtraArgs(s.crOptions.extraArgs...))

		testing.ContextLog(ctx, "Creating Chrome")
		var err error
		s.cr, err = chrome.New(ctx, opts...)
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

	// ARC, if requested.
	if s.useARC != nil {
		if err := func() error {
			ctx, cancel := context.WithTimeout(ctx, s.useARC.timeout)
			defer cancel()

			testing.ContextLog(ctx, "Creating ARC")
			var err error
			s.arc, err = arc.New(ctx, st.OutDir())
			if err != nil {
				return errors.Wrap(err, "failed to start ARC")
			}
			s.arcSnapshot, err = arc.NewSnapshot(ctx, s.arc)
			if err != nil {
				return errors.Wrap(err, "failed to take ARC state snapshot")
			}
			arc.Lock()
			return nil
		}(); err != nil {
			return err
		}
	}

	// Crostini, if requested.
	if s.useCrostini != nil {
		if err := func() error {
			ctx, cancel := context.WithTimeout(ctx, s.useCrostini.timeout)
			defer cancel()

			testing.ContextLog(ctx, "Creating Crostini")
			iOptions := crostini.GetInstallerOptions(st, true, s.useCrostini.debianVersion, s.useCrostini.largeContainer, s.cr.User())
			iOptions.UserName = s.cr.User()
			iOptions.MinDiskSize = s.useCrostini.minDiskSize
			if _, err := cui.InstallCrostini(ctx, s.tconn, iOptions); err != nil {
				return errors.Wrap(err, "failed to install Crostini")
			}
			var err error
			s.crostini, err = vm.DefaultContainer(ctx, s.cr.User())
			if err != nil {
				return errors.Wrap(err, "failed to connect to running container")
			}
			vm.Lock()
			return nil
		}(); err != nil {
			return err
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

	// ARC.
	if s.arc != nil {
		if err := s.arcSnapshot.Restore(ctx, s.arc); err != nil {
			return errors.Wrap(err, "failed to restore ARC")
		}
	}

	// Crostini.
	if s.crostini != nil {
		if err := crostini.BasicCommandWorks(ctx, s.crostini); err != nil {
			return errors.Wrap(err, "failed checking Crostini")
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

	if s.arc != nil {
		defer func() {
			arc.Unlock()
			if err := s.arc.Close(); err != nil {
				if errRet == nil {
					errRet = errors.Wrap(err, "failed to close ARC")
				} else {
					testing.ContextLog(ctx, "Failed to close ARC")
				}
			}
			s.arc = nil
		}()
	}

	if s.crostini != nil {
		defer func() {
			vm.Unlock()
			// How to stop crostini? Crostini's precondition tears down concierge, which seems a bit much
			if err := s.crostini.VM.Stop(ctx); err != nil {
				if errRet == nil {
					errRet = errors.Wrap(err, "failed to deactivate Crostini")
				} else {
					testing.ContextLog(ctx, "Failed to deactivate Crostini: ", err)
				}
			}
			s.crostini = nil
		}()
	}

	return nil
}
