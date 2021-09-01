// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"path/filepath"
	"time"

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
//		SoftwareDeps: []string{"vm_host", "dlc"},
//		Fixture: "vmDLC",
//	})
//
// Later, in the main test function, the VM artifacts are available via PreData.

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vmDLC",
		Desc: "Vm dlc is available",
		Contacts: []string{
			"woodychow@google.com",
			"crosvm-core@google.com",
		},
		Impl:            &dlcFixture{},
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    1 * time.Second,
		TearDownTimeout: 5 * time.Second,
	})
}

type dlcFixture struct {
}

func (f *dlcFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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

	return PreData{
		Kernel: filepath.Join(terminaDLCDir, "vm_kernel"),
		Rootfs: filepath.Join(terminaDLCDir, "vm_rootfs.img"),
	}
}

func (f *dlcFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := dlc.Purge(ctx, terminaDLCID); err != nil {
		s.Fatal("Purge failed: ", err)
	}
}

func (f *dlcFixture) Reset(ctx context.Context) error {
	return nil
}
func (f *dlcFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}
func (f *dlcFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
