// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui contains functions to interact with the ChromeOS parts of the crostini UI.
// This is primarily the settings and the installer.
package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/input"
)

const (
	// SizeB is a multiplier to convert bytes to bytes.
	SizeB = 1
	// SizeKB is a multiplier to convert bytes to kilobytes.
	SizeKB = 1024
	// SizeMB is a multiplier to convert bytes to megabytes.
	SizeMB = 1024 * 1024
	// SizeGB is a multiplier to convert bytes to gigabytes.
	SizeGB = 1024 * 1024 * 1024
	// SizeTB is a multiplier to convert bytes to terabytes.
	SizeTB = 1024 * 1024 * 1024 * 1024
)

const uiTimeout = 30 * time.Second

// Settings is a page object for the Crostini section of the settings app.
type Settings struct {
	tconn *chrome.TestConn
}

// Installer is a page object for the settings screen of the Crostini Installer.
type Installer struct {
	tconn *chrome.TestConn
}

// OpenSettings opens the settings app (if needed) and returns a settings page object.
//
// It also hides all notifications to ensure subsequent operations work correctly.
func OpenSettings(ctx context.Context, tconn *chrome.TestConn) (*Settings, error) {
	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to hide all notifications in OpenSettings()")
	}
	p := &Settings{tconn}
	err := p.ensureOpen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error in OpenSettings()")
	}
	return p, err
}

// ensureOpen checks if the settings app is open, and opens it if it is not.
func (p *Settings) ensureOpen(ctx context.Context) error {
	shown, err := ash.AppShown(ctx, p.tconn, apps.Settings.ID)
	if err != nil {
		return err
	}
	if shown {
		return nil
	}
	if err := apps.Launch(ctx, p.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch settings app")
	}
	if err := ash.WaitForApp(ctx, p.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "Settings app did not appear in the shelf")
	}
	return nil
}

// OpenInstaller clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  The returned Installer
// page object can be used to adjust the settings and to complete the installation.
func (p *Settings) OpenInstaller(ctx context.Context) (*Installer, error) {
	if err := p.ensureOpen(ctx); err != nil {
		return nil, errors.Wrap(err, "error in OpenInstaller()")
	}
	return &Installer{p.tconn}, uig.Do(ctx, p.tconn,
		uig.Steps(
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Linux (Beta)"}, uiTimeout).FocusAndWait(uiTimeout).LeftClick(),
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Next"}, uiTimeout).LeftClick()).WithNamef("OpenInstaller()"))
}

func parseDiskSizeString(str string) (uint64, error) {
	parts := strings.Split(str, " ")
	if len(parts) != 2 {
		return 0, errors.Errorf("could not parseDiskSizeString %s: does not have exactly 2 space separated parts", str)
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, errors.Wrapf(err, "could not parseDiskSizeString %s", str)
	}
	unitMap := map[string]float64{
		"B":  SizeB,
		"KB": SizeKB,
		"MB": SizeMB,
		"GB": SizeGB,
		"TB": SizeTB,
	}
	units, ok := unitMap[parts[1]]
	if !ok {
		return 0, errors.Errorf("could not parseDiskSizeString %s: does not have a recognized units string", str)
	}
	return uint64(num * units), nil
}

// SetDiskSize uses the slider on the Installer options pane to set the disk
// size to the smallest slider increment larger than the specified disk size.
func (p *Installer) SetDiskSize(ctx context.Context, minDiskSize uint64) error {
	window := uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeRootWebArea, Name: "Set up Linux (Beta) on your Chromebook"}, uiTimeout)
	slider := window.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeSlider}, uiTimeout)

	if err := uig.Do(ctx, p.tconn, slider.FocusAndWait(uiTimeout)); err != nil {
		return errors.Wrap(err, "error in SetDiskSize()")
	}

	getSize := func() (string, error) {
		node, err := uig.GetNode(ctx, p.tconn, slider.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText}, uiTimeout))
		if err != nil {
			return "", errors.Wrap(err, "error getting disk size setting")
		}
		defer node.Release(ctx)
		return node.Name, nil
	}

	// Use keyboard to manipulate the slider rather than writing
	// custom mouse code to click on exact locations on the slider.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error in SetDiskSize: error opening keyboard")
	}
	defer kb.Close()

	lastSize := uint64(0)
	for {
		sizeStr, err := getSize()
		if err != nil {
			return errors.Wrap(err, "error in SetDiskSize")
		}
		size, err := parseDiskSizeString(sizeStr)
		if err != nil {
			return errors.Wrap(err, "error in SetDiskSize")
		}
		if size > minDiskSize || size == lastSize {
			break
		}
		if size == lastSize {
			return errors.Errorf("error in SetDiskSize: could not set disk size to larger than %v, largest disk size available is %v (%v)", minDiskSize, sizeStr, size)
		}
		lastSize = size
		if err := kb.Accel(ctx, "right"); err != nil {
			return errors.Wrap(err, "error in SetDiskSize: error sending right arrow key")
		}
	}
	return nil
}

// Install clicks the install button and waits for the Linux installation to complete.
func (p *Installer) Install(ctx context.Context) error {
	// First check for an error screen.
	status, err := ui.Find(ctx, p.tconn, ui.FindParams{Role: ui.RoleTypeStatus})
	if err == nil {
		defer status.Release(ctx)
		// There is an error message, fetch and return it rather than the "can't find Install button" error.
		nodes, err := status.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeStaticText})
		if err != nil {
			return err
		}
		var messages []string
		for _, node := range nodes {
			messages = append(messages, node.Name)
			node.Release(ctx)
		}
		message := strings.Join(messages, ": ")
		if strings.HasPrefix(message, "Error") {
			return errors.Errorf("error message in dialog: %s", strings.Join(messages, ": "))
		}
	}
	return uig.Do(ctx, p.tconn,
		uig.Steps(
			uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Install"}, uiTimeout).LeftClick(),
			uig.WaitUntilDescendantGone(ui.FindParams{Role: ui.RoleTypeButton, Name: "Cancel"}, 10*time.Minute)).WithNamef("Install()"))
}
