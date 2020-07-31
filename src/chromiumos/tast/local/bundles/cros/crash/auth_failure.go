// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crash"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	crashAuthFailureBaseName = `auth_failure\.\d{8}\.\d{6}\.\d+\.0`
	crashAuthFailureMetaName = crashAuthFailureBaseName + `\.meta`
	sigAuthFailure           = "sig=.*-auth failure"
)

var expectedAuthFailureRegexes = []string{
	crashAuthFailureBaseName + `\.log`,
	crashAuthFailureMetaName,
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthFailure,
		Desc: "Verify auth failures are logged as expected",
		Contacts: []string{
			"chingkang@google.com",
			"cros-telemetry@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"tpm1"},
		Attr:         []string{"group:mainline"},
	})
}

func AuthFailure(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)
	tpmManager := helper.TPMManagerClient()
	daemonController := helper.DaemonController()

	opt := crash.WithMockConsent()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Restart TPM daemons first to cleanup previous auth failure
	if err := daemonController.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM daemons: ", err)
	}
	// Sleep for a while to prevent cryptohome command failing.
	testing.Sleep(ctx, time.Second)

	// Set up the crash test, ignoring non-auth-failure crashes.
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("auth_failure"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

	// Test if the well-known auth failure is blocked.
	if err := hwseclocal.IncreaseDAWithCheckVault(ctx, cryptohome, mountInfo); err != nil {
		s.Fatal("Failed to increase dictionary attcack counter: ", err)
	}
	// Restart TPM daemons to generate auth failure log
	if err := daemonController.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM daemons: ", err)
	}

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)

	files, err := crash.WaitForCrashFiles(ctx, crashDirs, expectedAuthFailureRegexes)
	if err == nil {
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[crashAuthFailureMetaName]...); err != nil {
			s.Error("Failed to save unexpected crashes: ", err)
		}
		s.Fatal("Found crash report for a well-known auth failure")
	}

	// Test if crash report is generated when tcsd find auth failure.
	if err := hwseclocal.IncreaseDAForTpm1(ctx, tpmManager); err != nil {
		s.Fatal("Failed to increase dictionary attcack counter: ", err)
	}
	// Restart TPM daemons to generate auth failure log
	if err := daemonController.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM daemons: ", err)
	}

	s.Log("Waiting for files")
	removeFilesCtx := ctx
	ctx, cancel = ctxutil.Shorten(removeFilesCtx, time.Second)
	defer cancel()
	files, err = crash.WaitForCrashFiles(ctx, crashDirs, expectedAuthFailureRegexes, crash.Timeout(time.Minute))
	if err != nil {
		s.Fatal("Failed to find expected files: ", err)
	}
	defer func(ctx context.Context) {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Error("Failed to clean up files: ", err)
		}
	}(removeFilesCtx)

	if len(files[crashAuthFailureMetaName]) == 1 {
		metaFile := files[crashAuthFailureMetaName][0]
		contents, err := ioutil.ReadFile(metaFile)
		if err != nil {
			s.Errorf("Failed to read meta file %s contents: %v", metaFile, err)
		} else {
			res, err := regexp.Match(sigAuthFailure, contents)
			if err != nil {
				s.Error("Failed to run regex: ", err)
			} else if !res {
				s.Error("Failed to find the expected auth_failure signature")
			}
			if err != nil || !res {
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			}
		}
	} else {
		s.Errorf("Unexpected number of files found, got %q; want 1", files[crashAuthFailureMetaName])
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[crashAuthFailureMetaName]...); err != nil {
			s.Error("Failed to save unexpected crashes: ", err)
		}
	}
}
