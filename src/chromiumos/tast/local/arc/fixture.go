// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "arcBooted",
		Desc: "ARC is booted",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedWithDisableSyncFlags is a fixture similar to arcBooted. The only difference from arcBooted is that ARC content sync is disabled to avoid noise during power/performance measurements.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithDisableSyncFlags",
		Desc: "ARC is booted with disabling sync flags",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			var opts = []chrome.Option{chrome.ARCEnabled()}
			for _, extraArg := range DisableSyncFlags() {
				opts = append(opts, chrome.ExtraArgs(extraArg))
			}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedInTabletMode is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched in tablet mode in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedInTabletMode",
		Desc: "ARC is booted in tablet mode",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.ExtraArgs("--enable-virtual-keyboard")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})
}

type bootedFixture struct {
	cr   *chrome.Chrome
	arc  *ARC
	init *Snapshot

	fOpt chrome.OptionsCallback // Function to return chrome options.
}

// NewArcBootedFixture returns a FixtureImpl with a OptionsCallback function provided.
func NewArcBootedFixture(fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{fOpt: fOpt}
}

func (f *bootedFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	opts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain fixture options: ", err)
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		if !success {
			cr.Close(ctx)
		}
	}()

	arc, err := New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if !success {
			arc.Close()
		}
	}()

	init, err := NewSnapshot(ctx, arc)
	if err != nil {
		s.Fatal("Failed to take ARC state snapshot: ", err)
	}

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this bootedFixture is active.
	Lock()
	chrome.Lock()

	f.cr = cr
	f.arc = arc
	f.init = init
	success = true
	return &PreData{
		Chrome: cr,
		ARC:    arc,
	}
}

func (f *bootedFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.arc.Close(); err != nil {
		s.Log("Failed to close ARC: ", err)
	}
	f.arc = nil

	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome: ", err)
	}
	f.cr = nil
}

func (f *bootedFixture) Reset(ctx context.Context) error {
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}
	return f.init.Restore(ctx, f.arc)
}

func (f *bootedFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.
}

func (f *bootedFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.

	if err := saveARCVMConsole(ctx, s.OutDir()); err != nil {
		s.Error("Failed to to save ARCVM console output: ", err)
	}

	if s.HasError() {
		faillogDir := filepath.Join(s.OutDir(), "faillog")
		if err := os.MkdirAll(faillogDir, 0755); err != nil {
			s.Error("Failed to make faillog/ directory: ", err)
			return
		}
		if err := saveProcessList(ctx, f.arc, faillogDir); err != nil {
			s.Error("Failed to save the process list in ARCVM: ", err)
		}
	}
}

// saveARCVMConsole saves the console output of ARCVM Kernel to the output directory using vm_pstore_dump command.
func saveARCVMConsole(ctx context.Context, outDir string) error {
	const (
		pstoreCommandPath                 = "/usr/bin/vm_pstore_dump"
		pstoreCommandExitCodeFileNotFound = 2
		arcvmConsoleName                  = "messages-arcvm"
	)

	// Do nothing for containers. The console output is already captured for containers.
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}
	if !isVMEnabled {
		return nil
	}

	// TODO(b/153934386): Remove this check when pstore is enabled on ARM.
	// The pstore feature is enabled only on x86_64. It's not enabled on some architectures, and this `vm_pstore_dump` command doesn't exist on such architectures.
	if _, err := os.Stat(pstoreCommandPath); os.IsNotExist(err) {
		testing.ContextLog(ctx, "Saving messages-arcvm file is skipped because vm_pstore_dump command is not found")
		return nil
	}

	path := filepath.Join(outDir, arcvmConsoleName)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := testexec.CommandContext(ctx, pstoreCommandPath)
	cmd.Stdout = file
	var errbuf bytes.Buffer
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		errmsg := errbuf.String()
		if cmd.ProcessState.ExitCode() == pstoreCommandExitCodeFileNotFound {
			// This failure sometimes happens when ARCVM failed to boot. So we don't make this error.
			testing.ContextLogf(ctx, "vm_pstore_dump command failed because the .pstore file doesn't exist: %#v", errmsg)
		} else {
			return errors.Wrapf(err, "vm_pstore_dump command failed with an unexpected reason: %#v", errmsg)
		}
	}
	return nil
}

func saveProcessList(ctx context.Context, a *ARC, outDir string) error {
	path := filepath.Join(outDir, "ps-arcvm.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := a.Command(ctx, "ps", "-AfZ")
	cmd.Stdout = file
	return cmd.Run()
}
