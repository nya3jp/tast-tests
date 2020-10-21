// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeInstallation,
		Desc:         "Test resizing during installation",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Vars:         []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{{
			Name:              "artifact",
			Val:               "artifact",
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           14 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Val:               "artifact",
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           14 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download",
			Val:     "download",
			Timeout: 20 * time.Minute,
		}},
		Pre: chrome.LoggedIn(),
	})
}

func ResizeInstallation(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	mode := ui.Artifact
	if strings.HasPrefix(s.Param().(string), ui.Download) {
		mode = ui.Download
	}

	iOptions := &ui.InstallationOptions{
		UserName:    cr.User(),
		Mode:        mode,
		MinDiskSize: 16 * settings.SizeGB,
	}
	if mode == ui.Artifact {
		iOptions.ImageArtifactPath = s.DataPath(crostini.ImageArtifact)
	}

	// Cleanup.
	defer func() {
		// Stop concierge.
		if _, err = vm.GetRunningContainer(ctx, cr.User()); err != nil {
			testing.ContextLogf(ctx, "Failed to connect to the container, it might not exist: %s", err)
		} else if err := vm.StopConcierge(ctx); err != nil {
			testing.ContextLogf(ctx, "Failure stopping concierge: %s", err)
		}

		// Unmount the component.
		vm.UnmountComponent(ctx)
		if err := vm.DeleteImages(); err != nil {
			testing.ContextLogf(ctx, "Error deleting images: %q", err)
		}
	}()

	// Install Crostini.
	if err := ui.InstallCrostini(ctx, tconn, iOptions); err != nil {
		s.Fatal("Failed to install Crostini: ", err)
	}

	// Launch Terminal
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to lauch terminal after installing Crostini: ", err)
	}
	defer terminalApp.Close(ctx)

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, iOptions.UserName)
	if err != nil {
		s.Fatal("Failed to connect to the container: ", err)
	}

	if err := verifyDiskSize(ctx, tconn, cont, "16.0 GB", iOptions.MinDiskSize); err != nil {
		s.Fatal("Failed to verify disk size: ", err)
	}

}

func verifyDiskSize(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, sizeInString string, size uint64) error {
	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Linux Settings")
	}
	defer st.Close(ctx)

	// Check the disk size on the Settings app.
	sizeOnSettings, err := st.GetDiskSize(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the disk size from the Settings app")
	}
	if sizeInString != sizeOnSettings {
		return errors.Wrapf(err, "failed to verify the disk size on the Settings app, got %s, want %s", sizeOnSettings, sizeInString)
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
			return errors.Errorf("failed to verify disk size, got %d, want approximately %d", contSize, size)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify the disk size of the container")
	}

	return nil
}
