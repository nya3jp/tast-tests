// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiskResizing,
		Desc:         "Tests that the VM disk can be resized from settings",
		Contacts:     []string{"sidereal@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
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

func DiskResizing(ctx context.Context, s *testing.State) {
	chrome := s.PreValue().(crostini.PreData).Chrome
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	userName := strings.Split(chrome.User(), "@")[0]
	defer crostini.RunCrostiniPostTest(ctx, cont)

	terminalApp, err := terminalapp.Launch(ctx, tconn, userName)
	if err != nil {
		s.Fatal("Failed to launch terminal: ", err)
	}

	initialSize, err := getDiskSize(ctx, cont)
	if err != nil {
		s.Fatal("Failed to get initial disk size: ", err)
	}

	if err := resizeDisk(ctx, tconn, keyboard, true); err != nil {
		s.Fatal("Failed to resize disk: ", err)
	}

	newSize, err := getDiskSize(ctx, cont)
	if err != nil {
		s.Fatal("Failed to get new disk size: ", err)
	}
	if newSize <= initialSize {
		s.Fatalf("Failed to increase disk size: was %d, now %d", initialSize, newSize)
	}

	if err := terminalApp.ShutdownCrostini(ctx); err != nil {
		s.Fatal("Failed to shutdown crostini: ", err)
	}

	if err := resizeDisk(ctx, tconn, keyboard, false); err != nil {
		s.Fatal("Failed to resize disk: ", err)
	}

	if _, err := terminalapp.Launch(ctx, tconn, userName); err != nil {
		s.Fatal("Failed to relaunch terminal: ", err)
	}
	newSize2, err := getDiskSize(ctx, cont)
	if err != nil {
		s.Fatal("Failed to get new disk size: ", err)
	}
	if newSize2 >= newSize {
		s.Fatalf("Failed to decrease disk size: was %d, now %d", newSize, newSize2)
	}
}

func getDiskSize(ctx context.Context, cont *vm.Container) (int64, error) {
	output, err := cont.Command(ctx, "df", "--output=size", "--block-size=1", "/").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to run df")
	}
	var result int64
	var header string
	_, err = fmt.Sscanf(string(output), "%s\n%d", &header, &result)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse df output")
	}
	return result, nil
}

func resizeDisk(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, increase bool) error {
	const uiTimeout = 30 * time.Second

	app, err := settings.OpenLinuxSettings(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open settings app")
	}
	defer app.Close(ctx)

	if err := uig.Do(ctx, tconn, uig.Steps(
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Change disk size"}, uiTimeout).FocusAndWait(uiTimeout).LeftClick(),
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeSlider}, uiTimeout).FocusAndWait(uiTimeout),
	)); err != nil {
		return errors.Wrap(err, "failed to locate disk size slider")
	}

	var key string
	if increase {
		key = "right"
	} else {
		key = "left"
	}

	for i := 0; i < 5; i++ {
		if err := keyboard.Accel(ctx, key); err != nil {
			return errors.Wrapf(err, "failed to press key %q", key)
		}
	}

	if err := uig.Do(ctx, tconn, uig.Steps(
		uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Resize"}, uiTimeout).FocusAndWait(uiTimeout).LeftClick(),
		uig.WaitUntilDescendantGone(ui.FindParams{Role: ui.RoleTypeButton, Name: "Resize"}, uiTimeout),
	)); err != nil {
		return errors.Wrap(err, "failed to click resize")
	}

	return nil
}
