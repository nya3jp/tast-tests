// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/devicemode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerTablet",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in tablet mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.TabletMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		Parent:          "crostiniBusterLargeContainer",
		Vars:            []string{"keepState"},
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerClamshell",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in clamshell mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.ClamshellMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		Parent:          "crostiniBusterLargeContainer",
		Vars:            []string{"keepState"},
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})
}

// crostiniAppsFixture holds the runtime state of the fixture.
type crostiniAppsFixture struct {
	tconn            *chrome.TestConn
	deviceMode       devicemode.DeviceMode
	revertDeviceMode func(ctx context.Context) error
}

func (f *crostiniAppsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.tconn = s.ParentValue().(FixtureData).Tconn
	return s.ParentValue().(FixtureData)
}

func (f *crostiniAppsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	revert, err := devicemode.EnsureDeviceMode(ctx, f.tconn, f.deviceMode)
	if err != nil {
		s.Logf("Failed to set device mode to %s : %s", f.deviceMode, err)
	}
	f.revertDeviceMode = revert
}

func (f *crostiniAppsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.revertDeviceMode != nil {
		if err := f.revertDeviceMode(ctx); err != nil {
			s.Log("Failed to reset device mode: ", err)
		}
		f.revertDeviceMode = nil
	}
}

func (f *crostiniAppsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (f *crostiniAppsFixture) Reset(ctx context.Context) error {
	return nil
}
