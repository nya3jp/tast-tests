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
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TwoUsersInstall,
		Desc:         "Test two users can install crostini separately",
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
			Name:    "download_stretch",
			Val:     "download",
			Timeout: 20 * time.Minute,
		}, {
			Name:    "download_buster",
			Val:     "download",
			Timeout: 20 * time.Minute,
		}},
	})
}

func TwoUsersInstall(ctx context.Context, s *testing.State) {
	mode := cui.Artifact
	if strings.HasPrefix(s.Param().(string), cui.Download) {
		mode = cui.Download
	}

	// Login options for the first user.
	optsUser1 := []chrome.Option{chrome.Auth(s.RequiredVar("crostini.gaiaUsername"), s.RequiredVar("crostini.gaiaPassword"), s.RequiredVar("crostini.gaiaID")),
		chrome.ExtraArgs("--vmodule=crostini*=1"),
		chrome.GAIALogin()}

	// Prepare to install Crostini for the first user.
	var firstCr *chrome.Chrome
	var firstTconn *chrome.TestConn
	var err error
	if firstCr, firstTconn, err = loginChromeAndGetTconn(ctx, optsUser1...); err != nil {
		s.Fatalf("Failed to login Chrome and get test API for %s: %s", s.RequiredVar("crostini.gaiaUsername"), err)
	}

	iOptionsUser1 := &cui.InstallationOptions{
		UserName: firstCr.User(),
		Mode:     mode,
	}
	if mode == cui.Artifact {
		iOptionsUser1.ImageArtifactPath = s.DataPath(crostini.ImageArtifact)
	}
	// Cleanup for the first user.
	defer func() {
		if err = cleanup(ctx, optsUser1...); err != nil {
			s.Fatalf("Failed to uninstall Crostini for %s: %s", iOptionsUser1.UserName, err)
		}
	}()

	// Install Crostini and shut it down.
	if err = installAndShutDown(ctx, firstTconn, iOptionsUser1); err != nil {
		s.Fatalf("Failed to test Crostini for %s: %s", iOptionsUser1.UserName, err)
	}

	// Login options for the second user.
	optsUser2 := chrome.ExtraArgs("--vmodule=crostini*=1")
	var secondCr *chrome.Chrome
	var secondTconn *chrome.TestConn
	// Prepare to install Crostini for the second user.
	if secondCr, secondTconn, err = loginChromeAndGetTconn(ctx, optsUser2); err != nil {
		s.Fatal("Failed to login Chrome and get test API for testuser: ", err)
	}

	iOptionsUser2 := &cui.InstallationOptions{
		UserName: secondCr.User(),
		Mode:     mode,
	}
	if mode == cui.Artifact {
		iOptionsUser2.ImageArtifactPath = s.DataPath(crostini.ImageArtifact)
	}
	// Cleanup for the second user.
	defer func() {
		if err = cleanup(ctx, optsUser2); err != nil {
			s.Fatalf("Failed to uninstall Crostini for %s: %s", iOptionsUser2.UserName, err)
		}
	}()

	// Install Crostini and shut it down.
	if err = installAndShutDown(ctx, secondTconn, iOptionsUser2); err != nil {
		s.Fatalf("Failed to test Crostini for user %s: %s", iOptionsUser2.UserName, err)
	}

}

func loginChromeAndGetTconn(ctx context.Context, opts ...chrome.Option) (cr *chrome.Chrome, tconn *chrome.TestConn, err error) {
	if cr, err = chrome.New(ctx, opts...); err != nil {
		return nil, nil, errors.Wrap(err, "failed to login Chrome")
	}
	if tconn, err = cr.TestAPIConn(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to get test API connection")
	}

	return cr, tconn, nil
}

func cleanup(ctx context.Context, opts ...chrome.Option) (err error) {
	// Login.
	var cr *chrome.Chrome
	if cr, _, err = loginChromeAndGetTconn(ctx, opts...); err != nil {
		return errors.Wrap(err, "failed to login Chrome")
	}

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, cr.User())
	if err != nil {
		testing.ContextLogf(ctx, "Failed to connect to the container, it might not exist: %s", err)
	}

	if cont != nil {
		if err := vm.StopConcierge(ctx); err != nil {
			testing.ContextLogf(ctx, "Failure stopping concierge: %q", err)
		}
		cont = nil
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
		cr = nil
	}

	return nil
}

func installAndShutDown(ctx context.Context, tconn *chrome.TestConn, iOptions *cui.InstallationOptions) error {
	// Install Crostini.
	if err := cui.InstallCrostini(ctx, tconn, iOptions); err != nil {
		return errors.Wrapf(err, "failed to install Crostini for user %s", iOptions.UserName)
	}

	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(iOptions.UserName, "@")[0])
	if err != nil {
		return errors.Wrapf(err, "failed to lauch terminal after installing Crostini for user %s", iOptions.UserName)
	}
	defer terminalApp.Close(ctx)

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, iOptions.UserName)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to connect to the container, it might not exist: %s", err)
	}

	// Shutdown Crostini.
	if err := terminalApp.ShutdownCrostini(ctx, cont); err != nil {
		return errors.Wrapf(err, "failed to shutdown Crostini after installing it for user %s", iOptions.UserName)
	}

	return nil
}
