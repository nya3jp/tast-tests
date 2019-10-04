// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Goofy,
		Desc:     "Setup factory toolkit and exercise Goofy with custom TestList",
		Contacts: []string{"menghuan@chromium.org", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{"factory_image.zip"},
	})
}

func Goofy(ctx context.Context, s *testing.State) {
	const testListName = "generic_tast"
	const finishFlagFilePath = "/tmp/tast_factory_test"
	const timeout = 2 * time.Minute

	setupFactory(ctx, s)
	defer cleanup(ctx, s)

	if err := setTestList(ctx, testListName); err != nil {
		s.Fatal("Failed to set TestList: ", err)
	}
	s.Logf("TestList is set to %s", testListName)

	// Make sure the temp file is not there
	os.Remove(finishFlagFilePath)
	if err := testexec.CommandContext(ctx, "start", "factory").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start factory toolkit: ", err)
	}
	s.Log("factory toolkit started, waiting for TestList finished")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ioutil.ReadFile(finishFlagFilePath); err != nil {
			return errors.Wrapf(err, "failed to read %s", finishFlagFilePath)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Fatal("Failed to execute the TestList: ", err)
	}
}

func setupFactory(ctx context.Context, s *testing.State) {
	if ver, err := installFactoryToolKit(ctx, s.DataPath("factory_image.zip")); err != nil {
		s.Fatal("Install fail: ", err)
	} else {
		s.Logf("Installed factory toolkit with version: %s", string(ver))
	}
}

func cleanup(ctx context.Context, s *testing.State) {
	s.Log("Cleanup DUT")

	stopGoofy(ctx)
	s.Log("stopped Goofy")
	uninstallFactoryToolKit(ctx)
	s.Log("uninstalled factory toolkit")
}

func installFactoryToolKit(ctx context.Context, imagePath string) (string, error) {
	// Create an temp dir
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	// unzip toolkit installer from image
	if err := testexec.CommandContext(ctx, "unzip", imagePath, "toolkit/install_factory_toolkit.run", "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip factory toolkit from factory_image.zip")
	}

	// Install factory toolkit
	toolkitPath := filepath.Join(tempDir, "toolkit/install_factory_toolkit.run")
	if err := testexec.CommandContext(ctx, toolkitPath, "--", "--yes").Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to install factory toolkit")
	}

	// Get the factory toolkit version
	b, err := ioutil.ReadFile("/usr/local/factory/TOOLKIT_VERSION")
	if err != nil {
		return "", errors.Wrap(err, "failed to read version file")
	}
	return string(b), nil
}

func uninstallFactoryToolKit(ctx context.Context) error {
	return testexec.CommandContext(ctx, "factory_uninstall", "--yes").Run(testexec.DumpLogOnError)
}

// stopGoofy and cleanup all factory configuration
func stopGoofy(ctx context.Context) error {
	return testexec.CommandContext(ctx, "factory_restart", "-S", "-a").Run(testexec.DumpLogOnError)
}

// setTestList set the default TestList to `testListID` via factory test-list
// command.
func setTestList(ctx context.Context, testListID string) error {
	const testListConfigPath = "/usr/local/factory/py/test/test_lists/active_test_list.json"

	if err := testexec.CommandContext(ctx, "factory", "test-list", testListID).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute `factory test-list` command")
	}

	// Verify the config content is correct.
	if b, err := ioutil.ReadFile(testListConfigPath); err != nil {
		return errors.Wrap(err, "failed to read TestList config file")
	} else if !strings.Contains(string(b), testListID) {
		return errors.Wrap(err, "testList config file doesn't contain the correct test id")
	} else {
		return nil
	}
}
