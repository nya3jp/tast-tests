// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "arcBooted",
		Desc:            "ARC is booted",
		Impl:            &bootedFixture{},
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type bootedFixture struct {
	cr   *chrome.Chrome
	arc  *ARC
	init *Snapshot
}

func (f *bootedFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
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
	// TODO(nya): Should we also check that p.cr is still usable?
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
}

// saveARCVMConsole saves the console output of ARCVM Kernel to the output directory using vm_pstore_dump command.
func saveARCVMConsole(ctx context.Context, outDir string) error {
	const (
		pstoreCommandPath = "/usr/bin/vm_pstore_dump"
		arcvmConsoleName  = "arcvm-console.txt"
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
	return cmd.Run()
}
