// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ScreenDiffFixtureName is the name of the fixture returned by ScreenDiffFixture()
const ScreenDiffFixtureName = "screenDiff"

const goldServiceAccountKeyFile = "/tmp/gold_service_account_key.json"
const goldServiceAccountKeyVar = "goldctl.goldServiceAccountKey"

// TODO(crbug.com/skia/10808): Change this once we have a production instance.
const goldInstance = "cros-tast-dev"

// TODO(crbug.com/skia/10808): Remove this once we have a production instance that relies on a unique identifier that doesn't use commit IDs (see the getCrosVersion function).
const commitID = "1234567890abcdef"

type basicState interface {
	Fatal(args ...interface{})
}

type varState interface {
	basicState
	Var(name string) (string, bool)
}

func getCrosVersion() (string, error) {
	contents, err := ioutil.ReadFile("/etc/lsb-release")
	if err != nil {
		return "", err
	}
	kvs := map[string]string{}
	for _, line := range strings.Split(string(contents), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 {
			kvs[kv[0]] = kv[1]
		}
	}
	return fmt.Sprintf("%s.%s", kvs["CHROMEOS_RELEASE_CHROME_MILESTONE"], kvs["CHROMEOS_RELEASE_BUILD_NUMBER"]), nil
}

func authenticateGold(ctx context.Context, st varState) {
	// Write the service account key for screenshot tests to a file.
	key, ok := st.Var(goldServiceAccountKeyVar)
	if !ok {
		st.Fatal(errors.New("couldn't get the gold service account key. Please ensure you have access to tast-tests-private"))
	}
	if err := ioutil.WriteFile(goldServiceAccountKeyFile, []byte(key), 0644); err != nil {
		st.Fatal(err)
	}

	if err := runGoldCommand("auth", "--service-account", goldServiceAccountKeyFile); err != nil {
		st.Fatal(err)
	}
}

func uploadDiffsWithoutAuthentication(ctx context.Context, st basicState) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		st.Fatal(errors.New("couldn't get output directory containing screenshots"))
	}
	dir = filepath.Join(dir, "screenshots")
	tests, err := ioutil.ReadDir(dir)
	if err != nil {
		st.Fatal(err)
	}
	var failedDiffs = []string{}
	for _, test := range tests {
		testDir := filepath.Join(dir, test.Name())
		if err := runGoldCommand("imgtest", "init", "--instance", goldInstance, "--keys-file", filepath.Join(testDir, KeysFile), "--commit", commitID, "--passfail"); err != nil {
			st.Fatal(err)
		}

		if err := runGoldCommand("imgtest", "add", "--test-name", test.Name(), "--png-file", filepath.Join(testDir, ScreenshotFile)); err != nil {
			failedDiffs = append(failedDiffs, fmt.Sprintf("Failed screendiff test %s\n%s", test.Name(), err.Error()))
		}
	}
	if len(failedDiffs) > 0 {
		st.Fatal(errors.New("\n\n" + strings.Join(failedDiffs, "\n\n")))
	}
}

func runGoldCommand(subcommand string, args ...string) error {
	args = append([](string){subcommand, "--work-dir", "/tmp/goldctl"}, args...)
	cmd := exec.Command("goldctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The suggested method of writing to a file to avoid newlines is a bad idea for our case since they're errors the user needs to immediately see.
		err = errors.Errorf("while running goldctl command %s\n%s", args, string(out)) // NOLINT
	}
	return err
}

// UploadGoldDiffs authenticates and uploads the screenshots that have been taken to the diffing service gold.
func UploadGoldDiffs(ctx context.Context, st varState) {
	authenticateGold(ctx, st)
	uploadDiffsWithoutAuthentication(ctx, st)
}

type screenshotTestFixtureImpl struct {
}

func (f *screenshotTestFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	authenticateGold(ctx, s)
	return nil
}

func (f *screenshotTestFixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *screenshotTestFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	uploadDiffsWithoutAuthentication(ctx, s)
}

func (f *screenshotTestFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (f *screenshotTestFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState)    {}

// ScreenDiffFixture returns a fixture that automatically uploads any screenshots taken by the screenshot.Diff() method to gold.
func ScreenDiffFixture() *testing.Fixture {
	return &testing.Fixture{
		Name: ScreenDiffFixtureName,
		Desc: "A fixture that automatically uploads any screenshots taken by the screenshot.Diff() method to gold.",
		Impl: &screenshotTestFixtureImpl{},
		Vars: []string{goldServiceAccountKeyVar},
	}
}
