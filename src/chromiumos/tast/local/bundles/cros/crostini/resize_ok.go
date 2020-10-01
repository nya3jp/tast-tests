// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeOk,
		Desc:         "Test resizing disk of Crostini from the Settings app",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
	})
}

func ResizeOk(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	keyboard := pre.Keyboard

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, pre)

	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Linux Settings: ", err)
	}
	defer st.Close(cleanupCtx)

	size, sizeOnSlider, err := resize(ctx, tconn, keyboard, st)
	if err != nil {
		s.Fatal("Failed to resize through moving slider: ", err)
	}

	// Check the disk size on the Settings app.
	sizeOnSettings, err := st.GetDiskSize(ctx)
	if err != nil {
		s.Fatal("Failed to get the disk size from the Settings app after resizing: ", err)
	}
	if sizeOnSlider != sizeOnSettings {
		s.Fatalf("Failed to verify the disk size on the Settings app, got %s, want %s", sizeOnSettings, sizeOnSlider)
	}
	// Check the disk size of the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		disk, err := pre.Container.VM.Concierge.GetVMDiskInfo(ctx, vm.DefaultVMName)
		if err != nil {
			return errors.Wrap(err, "failed to get VM disk info")
		}
		contSize := disk.GetSize()

		// Allow some gap.
		diff := size - contSize
		if size < contSize {
			diff = contSize - size
		}
		if diff > settings.SizeMB {
			return errors.Errorf("failed to verify disk size after cancelling resizing, got %d, want %d", contSize, size)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to verify the disk size of the container after resizing: ", err)
	}
}

func resize(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, st *settings.Settings) (uint64, string, error) {
	// Click Resize on Linux settings page.
	resizeDlg, err := st.ClickResize(ctx)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to click button Change on Linux settings page")
	}

	// Get the dialog node and params.
	dialog, err := uig.GetNode(ctx, tconn, resizeDlg.Self)
	if err != nil {
		return 0, "", errors.New("failed to get the parameter of the Resize Linux diski dialog")
	}
	dialogParams := ui.FindParams{
		Role: dialog.Role,
		Name: dialog.Name,
	}

	// Focus on the slider.
	if err := uig.Do(ctx, tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Slider.FocusAndWait(15*time.Second)); err != nil {
		return 0, "", errors.Wrap(err, "failed to focus on the slider on the Resize Linux disk dialog")
	}

	// Get the minimum size.
	minSize, err := settings.ChangeDiskSize(ctx, tconn, keyboard, resizeDlg.Slider, false, 0)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to resize to the minimum disk size")
	}
	// Get the maximum size.
	maxSize, err := settings.ChangeDiskSize(ctx, tconn, keyboard, resizeDlg.Slider, true, 500*settings.SizeGB)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to resize to the maximum disk size")
	}

	// Resize to the medium size.
	size, err := settings.ChangeDiskSize(ctx, tconn, keyboard, resizeDlg.Slider, false, minSize+(maxSize-minSize)/2)
	if err != nil {
		return 0, "", errors.Wrapf(err, "failed to resize to %s: ", minSize+(maxSize-minSize)/2)
	}

	// Record the new size on the slider.
	node, err := uig.GetNode(ctx, tconn, resizeDlg.Slider.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText}, 15*time.Second))
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to read the disk size from slider after resizing")
	}
	defer node.Release(ctx)

	if err := uig.Do(ctx, tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Resize.LeftClick()); err != nil {
		return 0, "", errors.Wrap(err, "failed to click button Resize on Resize Linux disk dialog")
	}

	// Wait the resize dialog gone.
	if err := ui.WaitUntilGone(ctx, tconn, dialogParams, 15*time.Second); err != nil {
		return 0, "", errors.Wrap(err, "failed to close the Resize Linux disk dialog")
	}
	return size, node.Name, nil
}
