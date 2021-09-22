// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides vmDLC fixture
package dlc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/testing"
)

const (
	// terminaDLCID is the id of termina in DLC.
	terminaDLCID = "termina-dlc"
)

// vmDLC is a fixture such as the guest kernel from DLC is installed before the test runs.
//
//	testing.AddTest(&testing.Test{
//		...
//		SoftwareDeps: []string{"vm_host", "dlc", "chrome"},
//		Fixture: "vmDLC",
//	})
//
// Later, in the main test function, the VM artifacts are available via FixtData.

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vmDLC",
		Desc: "Vm dlc is available",
		Contacts: []string{
			"woodychow@google.com",
			"crosvm-core@google.com",
		},
		Vars:            []string{"ui.signinProfileTestExtensionManifestKey"},
		Impl:            &dlcFixture{},
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    1 * time.Second,
		TearDownTimeout: 5 * time.Second,
	})
}

type dlcFixture struct {
	cr *chrome.Chrome
}

// The FixtData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(dlc.FixtData)
//		...
//	}
type FixtData struct {
	Kernel string // Path to the guest kernel.
	Rootfs string // Path to the guest rootfs image.
}

func (f *dlcFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// NoLogin is used to land in signin screen.
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	// Skip OOBE before Login screen.
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE connection: ", err)
	}
	if err := oobeConn.Close(); err != nil {
		s.Fatal("Failed to close OOBE connection: ", err)
	}
	f.cr = cr

	if err := dlc.Install(ctx, terminaDLCID, "" /*omahaURL*/); err != nil {
		s.Fatal("Failed to install DLC: ", err)
	}

	dlcMap, err := dlc.List(ctx)
	if err != nil {
		s.Fatal("Failed to list installed DLC(s): ", err)
	}
	infoList, ok := dlcMap[terminaDLCID]
	if !ok {
		s.Fatal("Something went wrong with the installation of " + terminaDLCID)
	}
	if len(infoList) != 1 {
		s.Fatalf("Exactly 1 %s should be available", terminaDLCID)
	}
	terminaDLCDir := infoList[0].RootMount

	return FixtData{
		Kernel: filepath.Join(terminaDLCDir, "vm_kernel"),
		Rootfs: filepath.Join(terminaDLCDir, "vm_rootfs.img"),
	}
}

func (f *dlcFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := dlc.Purge(ctx, terminaDLCID); err != nil {
		s.Fatal("Purge failed: ", err)
	}

	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *dlcFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	if err := dlc.Install(ctx, terminaDLCID, "" /*omahaURL*/); err != nil {
		return errors.Wrap(err, "failed to reinstall dlc")
	}
	return nil
}
func (f *dlcFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}
func (f *dlcFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
