// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/devicemode"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerTablet",
		Desc:            "Install Crostini with Buster in large container with apps installed in tablet mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.TabletMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBusterLargeContainer",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerClamshell",
		Desc:            "Install Crostini with Buster in large container with apps installed in clamshell mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.ClamshellMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBusterLargeContainer",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeLargeContainerTablet",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in tablet mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.TabletMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBullseyeLargeContainer",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("bullseye", true), GetContainerRootfsArtifact("bullseye", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeLargeContainerClamshell",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in clamshell mode",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.ClamshellMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBullseyeLargeContainer",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("bullseye", true), GetContainerRootfsArtifact("bullseye", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerTabletWithSnapshot",
		Desc:            "Install Crostini with Buster in large container with apps installed in tablet mode, take snapshot before test and restore it after test",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.TabletMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBusterLargeContainerSnapshot",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterLargeContainerClamshellWithSnapshot",
		Desc:            "Install Crostini with Buster in large container with apps installed in clamshell mode, take snapshot before test and restore it after test",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.ClamshellMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBusterLargeContainerSnapshot",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("buster", true), GetContainerRootfsArtifact("buster", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeLargeContainerTabletWithSnapshot",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in tablet mode, take snapshot before test and restore it after test",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.TabletMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBullseyeLargeContainerSnapshot",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("bullseye", true), GetContainerRootfsArtifact("bullseye", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeLargeContainerClamshellWithSnapshot",
		Desc:            "Install Crostini with Bullseye in large container with apps installed in clamshell mode, take snapshot before test and restore it after test",
		Contacts:        []string{"clumptini+oncall@google.com"},
		Impl:            &crostiniAppsFixture{deviceMode: devicemode.ClamshellMode},
		SetUpTimeout:    installationTimeout + uninstallationTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout + restartCrostiniTimeout,
		Parent:          "crostiniBullseyeLargeContainerSnapshot",
		Vars:            append([]string{"keepState"}, screenshot.ScreenDiffVars...),
		Data:            []string{GetContainerMetadataArtifact("bullseye", true), GetContainerRootfsArtifact("bullseye", true)},
	})
}

// crostiniAppsFixture holds the runtime state of the fixture.
type crostiniAppsFixture struct {
	cr               *chrome.Chrome
	tconn            *chrome.TestConn
	cont             *vm.Container
	kb               *input.KeyboardEventWriter
	deviceMode       devicemode.DeviceMode
	revertDeviceMode func(ctx context.Context) error
	screenRecorder   *uiauto.ScreenRecorder
	screenDiffer     *Screendiffer
}

func (f *crostiniAppsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	p := s.ParentValue().(FixtureData)
	f.tconn = p.Tconn
	f.cr = p.Chrome
	f.cont = p.Cont
	f.kb = p.KB
	f.screenDiffer = &Screendiffer{differ: nil, state: &screenDiffState{fixtState: s}}
	return FixtureData{p.Chrome, p.Tconn, p.Cont, p.KB, p.PostData, p.StartupValues, f.screenDiffer, p.DownloadsPath}
}

func (f *crostiniAppsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Setup the screen recorder.
	recorder, err := uiauto.NewScreenRecorder(ctx, f.tconn)
	if err != nil {
		s.Log("Failed to create screen recorder: ", err)
	}
	if recorder != nil {
		if err := recorder.Start(ctx, f.tconn); err != nil {
			s.Log("Failed to start screen recorder: ", err)
		}
		f.screenRecorder = recorder
	}

	// Setup the device mode.
	revert, err := devicemode.EnsureDeviceMode(ctx, f.tconn, f.deviceMode)
	if err != nil {
		s.Logf("Failed to set device mode to %s : %s", f.deviceMode, err)
	}
	f.revertDeviceMode = revert

	// Setup screendiff.
	f.screenDiffer.state.testState = s
	defaultWindowState := ash.WindowStateNormal
	if f.deviceMode == devicemode.TabletMode {
		// WindowStateNormal is invalid in the tablet mode.
		defaultWindowState = ash.WindowStateMaximized
	}

	// Using normalization&resizing doesn't help much to reduce the number of
	// untriaged images, as the apps are still rendered differently on
	// difference models despite of being the same size.
	// On the other hand, it may introduce some side-effects, i.e., it may make
	// eerything very small on the screen thus add difficulty for ui detection;
	// if not properly reverted, other tests sharing the same fixture will run
	// in the normalized state unexpectedly.
	screendiffConfig := screenshot.Config{
		DefaultOptions: screenshot.Options{
			SkipWindowResize: true,
			WindowState:      defaultWindowState,
			// Config fuzzy_max_different_pixels and fuzzy_pixel_delta_threshold
			// to help reduce the number of untriaged images, see go/goldctl for
			// parameter details.
			// Set these two parameters to allow 100-pixel difference with each
			// pixel being able differ to any degree.
			MaxDifferentPixels:  100,
			PixelDeltaThreshold: 255 * 4,
		},
		SkipDpiNormalization: true,
	}
	differ, err := screenshot.NewDifferFromChrome(ctx, f.screenDiffer.state, f.cr, screendiffConfig)
	if err != nil {
		s.Log("Failed to start screen differ: ", err)
	}
	f.screenDiffer.differ = &differ
}

func (f *crostiniAppsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if (*f.screenDiffer.differ) != nil {
		(*f.screenDiffer.differ).GetFailedDiffs()
	}

	if f.revertDeviceMode != nil {
		if err := f.revertDeviceMode(ctx); err != nil {
			s.Log("Failed to reset device mode: ", err)
		}
		f.revertDeviceMode = nil
	}
	if f.screenRecorder != nil {
		f.screenRecorder.StopAndSaveOnError(ctx, filepath.Join(s.OutDir(), "record.webm"), s.HasError)
	}

	// Restart Crostini in case of test failures to leave a clean env for the
	// following tests. This ensures all open apps are closed.
	if s.HasError() {
		// Open Terminal app.
		terminalApp, err := terminalapp.Launch(ctx, f.tconn)
		if err != nil {
			s.Log("Failed to open Terminal app: ", err)
		} else {
			if err := terminalApp.RestartCrostini(f.kb, f.cont, f.cr.NormalizedUser())(ctx); err != nil {
				s.Log("Failed to restart Crostini: ", err)
			}
		}
	}
}

func (f *crostiniAppsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (f *crostiniAppsFixture) Reset(ctx context.Context) error {
	return nil
}
