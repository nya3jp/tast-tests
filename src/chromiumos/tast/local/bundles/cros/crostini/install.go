// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/crostini"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Install,
		Desc:         "Test installation repeatedly",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Vars:         []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraData:         []string{vm.ArtifactData(), crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Timeout:           140 * time.Minute,
			},
		},
	})
}

func Install(ctx context.Context, s *testing.State) {
	// Login options for the first user.
	opts := []chrome.Option{chrome.ExtraArgs("--vmodule=crostini*=1")}

	failTimes := 0

	var failures []string

	for n := 0; n < 100; n++ {
		s.Log("Log==============: ", n)
		// Prepare to install Crostini for the first user.
		var cr *chrome.Chrome
		var tconn *chrome.TestConn
		var err error
		if cr, tconn, err = loginCT(ctx, opts...); err != nil {
			s.Fatalf("Failed to login Chrome and get test API for %s: %s", s.RequiredVar("crostini.gaiaUsername"), err)
		}

		iOptions := crostini.GetInstallerOptions(s, true /*isComponent*/, vm.DebianBuster, false /*largeContainer*/, cr.User())

		if err = install(ctx, tconn, cr, iOptions); err != nil {
			failTimes = failTimes + 1
			failures = append(failures, fmt.Sprintf("Error Msg: %s\n", err))
		}

		uninstallCrostini(ctx, tconn, cr)
	}

	s.Log("Results======================")
	s.Log("Failed times: ", failTimes)
	s.Log(failures)
	s.Log("Results======================")
}

func uninstallCrostini(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Linux Settings")
	}

	// Uninstall Crostini
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Remove Linux",
		st.ClickRemove(),
		ui.LeftClick(settings.RemoveConfirmDialog.Delete),
		ui.WaitUntilExists(settings.RemoveLinuxAlert),
		ui.WaitUntilGone(settings.RemoveLinuxAlert),
		ui.WaitUntilExists(settings.DevelopersButton))(ctx); err != nil {
		return err
	}

	if err := st.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close settings window after uninstalling Linux")
	}
	return nil
}

func loginCT(ctx context.Context, opts ...chrome.Option) (cr *chrome.Chrome, tconn *chrome.TestConn, err error) {
	if cr, err = chrome.New(ctx, opts...); err != nil {
		return nil, nil, errors.Wrap(err, "failed to login Chrome")
	}
	if tconn, err = cr.TestAPIConn(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to get test API connection")
	}

	return cr, tconn, nil
}

func uninstall(ctx context.Context, opts ...chrome.Option) error {
	// Login.
	cr, _, err := loginCT(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to login Chrome")
	}

	// Get the container.
	_, err = vm.GetRunningContainer(ctx, cr.User())
	if err != nil {
		testing.ContextLogf(ctx, "Failed to connect to the container, it might not exist: %s", err)
	} else {
		if err := vm.StopConcierge(ctx); err != nil {
			testing.ContextLogf(ctx, "Failure stopping concierge: %s", err)
		}
	}

	// Unmount the component.
	vm.UnmountComponent(ctx)
	if err := vm.DeleteImages(); err != nil {
		testing.ContextLogf(ctx, "Error deleting images: %q", err)
	}

	if cr != nil {
		if err := cr.Close(ctx); err != nil {
			return errors.Wrap(err, "failure closing chrome")
		}
	}

	return nil
}

func install(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, iOptions *cui.InstallationOptions) error {
	// Install Crostini.
	if _, err := cui.InstallCrostini(ctx, tconn, cr, iOptions); err != nil {
		return errors.Wrapf(err, "failed to install Crostini for user %s", iOptions.UserName)
	}

	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrapf(err, "failed to lauch terminal after installing Crostini for user %s", iOptions.UserName)
	}
	defer terminalApp.Close()(ctx)

	return nil
}
