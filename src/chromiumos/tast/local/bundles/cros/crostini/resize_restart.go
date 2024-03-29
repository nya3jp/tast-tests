// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeRestart,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test resizing disk of Crostini from the Settings app while Crostini is shutdown",
		Contacts:     []string{"clumptini+oncall@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func ResizeRestart(ctx context.Context, s *testing.State) {
	pre := s.FixtValue().(crostini.FixtureData)
	cr := pre.Chrome
	tconn := pre.Tconn
	cont := pre.Cont

	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open Linux Settings: ", err)
	}

	// Shutdown Crostini.
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch terminal: ", err)
	}
	if err := terminalApp.ShutdownCrostini(cont)(ctx); err != nil {
		s.Fatal("Failed to shutdown crostini: ", err)
	}

	curSize, targetSize, err := st.GetCurAndTargetDiskSize(ctx)
	if err != nil {
		s.Fatal("Failed to get current or target size: ", err)
	}

	// Resize.
	sizeOnSlider, size, err := st.Resize(ctx, targetSize)
	if err != nil {
		s.Fatal("Failed to resize through moving slider: ", err)
	}

	if _, err := terminalapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch terminal: ", err)
	}

	if err := st.VerifyResizeResults(ctx, cont, sizeOnSlider, size); err != nil {
		s.Fatal("Failed to verify resize results: ", err)
	}

	if err := terminalApp.Close()(ctx); err != nil {
		s.Fatal("Failed to close terminal: ", err)
	}

	// Resize back to the default value.
	sizeOnSlider, size, err = st.Resize(ctx, curSize)
	if err != nil {
		s.Fatal("Failed to resize back to the default value: ", err)
	}

	if err := st.VerifyResizeResults(ctx, cont, sizeOnSlider, size); err != nil {
		s.Fatal("Failed to verify resize results: ", err)
	}
}

func verifyResults(ctx context.Context, st *settings.Settings, cont *vm.Container, sizeOnSlider string, size uint64) error {
	// Check the disk size on the Settings app.
	sizeOnSettings, err := st.GetDiskSize(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the disk size from the Settings app after resizing")
	}
	if sizeOnSlider != sizeOnSettings {
		return errors.Wrapf(err, "failed to verify the disk size on the Settings app, got %s, want %s", sizeOnSettings, sizeOnSlider)
	}
	// Check the disk size of the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		disk, err := cont.VM.Concierge.GetVMDiskInfo(ctx, vm.DefaultVMName)
		if err != nil {
			return errors.Wrap(err, "failed to get VM disk info")
		}
		contSize := disk.GetSize()

		// Allow some gap.
		var diff uint64
		if size > contSize {
			diff = size - contSize
		} else {
			diff = contSize - size
		}
		if diff > settings.SizeMB {
			return errors.Errorf("failed to verify disk size after resizing, got %d, want approximately %d", contSize, size)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify the disk size of the container after resizing")
	}

	return nil
}
