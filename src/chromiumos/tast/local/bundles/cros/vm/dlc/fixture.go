// Copyright 2021 The ChromiumOS Authors
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
//		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
//		Fixture: "vmDLC",
//	})
//
// Later, in the main test function, the VM artifacts are available via FixtData.

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vmDLC",
		Desc: "Vm dlc is available",
		Contacts: []string{
			"keiichiw@chromium.org",
			"cros-virt-devices-guests@google.com",
		},
		Parent:          "chromeLoggedIn",
		Impl:            &dlcFixture{},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
	})
}

type dlcFixture struct {
}

// The FixtData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(dlc.FixtData)
//		...
//	}
type FixtData struct {
	Chrome *chrome.Chrome // Instance of Chrome
	Kernel string         // Path to the guest kernel.
	Rootfs string         // Path to the guest rootfs image.
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

	return FixtData{
		Chrome: s.ParentValue().(*chrome.Chrome),
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
	if err := dlc.Install(ctx, terminaDLCID, "" /*omahaURL*/); err != nil {
		return errors.Wrap(err, "failed to reinstall dlc")
	}
	return nil
}
func (f *dlcFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}
func (f *dlcFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
