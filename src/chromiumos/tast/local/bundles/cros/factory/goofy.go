// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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

func Goofy(fullCtx context.Context, s *testing.State) {
	const testListName = "generic_tast"
	const finishFlagFilePath = "/tmp/tast_factory_test"
	const timeout = 2 * time.Minute

	ctx, cancel := ctxutil.Shorten(fullCtx, 30*time.Second)
	defer cancel()

	setupFactory(ctx, s)
	defer cleanup(fullCtx, s)

	if err := setTestList(ctx, testListName); err != nil {
		s.Fatal("Failed to set TestList: ", err)
	}
	s.Logf("TestList is set to %s", testListName)

	// Make sure the temp file is not there.
	os.Remove(finishFlagFilePath)
	if err := testexec.CommandContext(ctx, "start", "factory").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start factory toolkit: ", err)
	}
	s.Log("factory toolkit started, waiting for TestList finished")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(finishFlagFilePath); err != nil {
			return errors.Wrapf(err, "failed to read %s", finishFlagFilePath)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Fatal("Failed to execute the TestList: ", err)
	}
}

func setupFactory(ctx context.Context, s *testing.State) {
	ver, err := installFactoryToolKit(ctx, s.DataPath("factory_image.zip"))
	if err != nil {
		s.Fatal("Install fail: ", err)
	}
	s.Logf("Installed factory toolkit with version: %s", ver)
}

func cleanup(ctx context.Context, s *testing.State) {
	s.Log("Start to cleanup DUT")

	if err := stopGoofy(ctx); err != nil {
		s.Fatal("Failed to stop goofy when cleanup: ", err)
	}
	s.Log("stopped Goofy")

	if err := uninstallFactoryToolKit(ctx); err != nil {
		s.Fatal("Failed to uninstall factory toolkit when cleanup: ", err)
	}
	s.Log("uninstalled factory toolkit")
}

func installFactoryToolKit(ctx context.Context, imagePath string) (version string, err error) {
	// Create an temp directory.
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	// Unzip toolkit installer from image.
	if err := testexec.CommandContext(ctx, "unzip", imagePath, "toolkit/install_factory_toolkit.run", "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip factory toolkit from factory_image.zip")
	}

	// Install factory toolkit.
	toolkitPath := filepath.Join(tempDir, "toolkit/install_factory_toolkit.run")
	if err := testexec.CommandContext(ctx, toolkitPath, "--", "--yes").Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to install factory toolkit")
	}

	// Get the factory toolkit version.
	b, err := ioutil.ReadFile("/usr/local/factory/TOOLKIT_VERSION")
	if err != nil {
		return "", errors.Wrap(err, "failed to read version file")
	}
	version = strings.TrimSpace(string(b))
	return
}

func uninstallFactoryToolKit(ctx context.Context) error {
	return testexec.CommandContext(ctx, "factory_uninstall", "--yes").Run(testexec.DumpLogOnError)
}

// stopGoofy and cleanup all factory configuration.
func stopGoofy(ctx context.Context) error {
	return testexec.CommandContext(ctx, "factory_restart", "-S", "-a").Run(testexec.DumpLogOnError)
}

// setTestList set the default TestList to `testListID` via factory test-list
// command.
func setTestList(ctx context.Context, testListID string) error {
	type DataFile struct {
		ID string `json:"id"`
	}
	const testListConfigPath = "/usr/local/factory/py/test/test_lists/active_test_list.json"
	var data DataFile

	if err := testexec.CommandContext(ctx, "factory", "test-list", testListID).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute `factory test-list` command")
	}

	// Verify the config content is correct.
	b, err := ioutil.ReadFile(testListConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read TestList config file")
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return errors.Wrap(err, "failed to decode JSON")
	}
	if data.ID != testListID {
		return errors.Wrap(err, "testList config file doesn't contain the correct test id")
	}

	return nil
}
