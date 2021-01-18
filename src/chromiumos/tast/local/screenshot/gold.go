// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ScreenDiffFixtureName is the name of the fixture returned by ScreenDiffFixture()
const ScreenDiffFixtureName = "screenDiff"

// GoldServiceAccountKeyVar contains the name of the variable storing the service account key.
const GoldServiceAccountKeyVar = "goldServiceAccountKey"

const goldServiceAccountKeyFile = "/tmp/gold_service_account_key.json"

// TODO(crbug.com/skia/10808): Change this once we have a production instance.
const goldInstance = "cros-tast-dev"

// TODO(crbug.com/skia/10808): Remove this once we have a production instance that relies on a unique identifier that doesn't use commit IDs (see the getCrosVersion function).
const commitID = "1234567890abcdef"

type varState interface {
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

func authenticateGold(ctx context.Context, st varState) error {
	// Write the service account key for screenshot tests to a file.
	key, ok := st.Var(goldServiceAccountKeyVar)
	if !ok {
		return errors.New("couldn't get the gold service account key. Please ensure you have access to tast-tests-private")
	}
	if err := ioutil.WriteFile(goldServiceAccountKeyFile, []byte(key), 0644); err != nil {
		return err
	}

	return runGoldCommand(ctx, "auth", "--service-account", goldServiceAccountKeyFile)
}

func uploadDiffsWithoutAuthentication(ctx context.Context) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("couldn't get output directory")
	}
	dir = filepath.Join(dir, "screenshots")
	tests, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrap(err, "couldn't read screenshot directory")
	}
	var failedDiffs = []string{}
	for _, test := range tests {
		name := test.Name()[:strings.LastIndex(test.Name(), "-")]
		testDir := filepath.Join(dir, test.Name())
		if err := runGoldCommand(ctx, "imgtest", "init", "--instance", goldInstance, "--keys-file", filepath.Join(testDir, KeysFile), "--commit", commitID, "--passfail"); err != nil {
			return err
		}

		if err := runGoldCommand(ctx, "imgtest", "add", "--test-name", name, "--png-file", filepath.Join(testDir, ScreenshotFile)); err != nil {
			failedDiffs = append(failedDiffs, err.Error())
		}
	}
	if len(failedDiffs) > 0 {
		// Each of these errors is the stdout from a failing diff.
		// Each failed diff will look something like this:
		// while running "goldctl imgtest --test-name blah --png-file blah.png"
		// Given image with hash <hash> for test blah
		// Expectation for test: <hash> (positive)
		// Untriaged or negative image: https://cros-tast-gold.skia.org/detail?test=blah&digest=<hash>
		// Test: blah FAIL
		return errors.New("\n\n" + strings.Join(failedDiffs, "\n\n"))
	}
	return nil
}

func runGoldCommand(ctx context.Context, subcommand string, args ...string) error {
	args = append([](string){subcommand, "--work-dir", "/tmp/goldctl"}, args...)
	cmd := testexec.CommandContext(ctx, "goldctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The suggested method of writing to a file to avoid newlines is irrelevant in this case,
		// since the stdout/err needs to be part of the error, which has newlines anyway.
		err = errors.Errorf("while running \"goldctl %s\"\n%s", strings.Join(args, " "), out) // NOLINT
	}
	return err
}

// UploadGoldDiffs authenticates and uploads the screenshots that have been taken to the diffing service gold.
func UploadGoldDiffs(ctx context.Context, st varState) error {
	if err := authenticateGold(ctx, st); err != nil {
		return err
	}
	return uploadDiffsWithoutAuthentication(ctx)
}

type screenshotTestFixtureImpl struct {
}

func (f *screenshotTestFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := authenticateGold(ctx, s); err != nil {
		s.Fatal("Failed to authenticate gold: ", err)
	}
	return nil
}

func (f *screenshotTestFixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *screenshotTestFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := uploadDiffsWithoutAuthentication(ctx); err != nil {
		s.Fatal("Failed during diffing: ", err)
	}
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
		// Authenticating on the gold server.
		SetUpTimeout: time.Duration(30) * time.Second,
		// Sending images to gold server.
		PostTestTimeout: time.Duration(5) * time.Minute,
	}
}
