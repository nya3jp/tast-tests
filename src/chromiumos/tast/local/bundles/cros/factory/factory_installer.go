// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FactoryInstaller,
		// TODO: fix me
		Desc:     "Demonstrates how to use artifact data files",
		Contacts: []string{"nya@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},

		Data: []string{"factory_image.zip"},
	})
}

func FactoryInstaller(ctx context.Context, s *testing.State) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	// Get the factory_image.zip artifact local path, which will be automatically
	// download from gs. It was uploaded by Chrome OS builder.
	factoryImagePath := s.DataPath("factory_image.zip")

	unzipCmd := testexec.CommandContext(ctx, "unzip", factoryImagePath, "toolkit/install_factory_toolkit.run", "-d", tempDir)
	if err := unzipCmd.Run(); err != nil {
		unzipCmd.DumpLog(ctx)
		s.Fatal("Unzip fail: ", err)
	}

	// Install factory toolkit
	toolkitPath := filepath.Join(tempDir, "toolkit/install_factory_toolkit.run")
	installCmd := testexec.CommandContext(ctx, toolkitPath, "--", "--yes")
	if err := installCmd.Run(); err != nil {
		installCmd.DumpLog(ctx)
		s.Fatal("Install fail fail: ", err)
	}
	defer os.RemoveAll("/usr/local/factory")

	if b, err := ioutil.ReadFile("/usr/local/factory/TOOLKIT_VERSION"); err != nil {
		s.Error("Failed reading file: ", err)
	} else {
		s.Log("Toolkit install successful: ", string(b))
	}
}
