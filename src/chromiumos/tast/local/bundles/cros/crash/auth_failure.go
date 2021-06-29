// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/hwsec"
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
		},
		SoftwareDeps: []string{"tpm1"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func AuthFailure(ctx context.Context, s *testing.State) {
	opt := crash.WithMockConsent()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Set up the crash test, ignoring non-auth-failure crashes.
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("auth_failure"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}
	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	// Generating auth failure by increasing DA counter.
	cmdRunner := hwsec.NewCmdRunner()
	helper, err := hwsec.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	tpmManager := helper.TPMManagerClient()

	err = hwsec.IncreaseDAForTpm1(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to increase dictionary attcack counter: ", err)
	}

	// Restart tcsd to generate auth failure log
	_, err = testexec.CommandContext(ctx, "restart", "tcsd").Output()
	if err != nil {
		s.Fatal("Failed to restart tcsd: ", err)
	}
	// Restart tpm_managerd to avoid tpm_managerd crashing when receiving next command, see b/192034446.
	// TODO(b/192034446): remove this once the problem is resolved.
	_, err = testexec.CommandContext(ctx, "restart", "tpm_managerd").Output()
	if err != nil {
		s.Fatal("Failed to restart tpm_managerd: ", err)
	}

	s.Log("Waiting for files")
	removeFilesCtx := ctx
	ctx, cancel = ctxutil.Shorten(removeFilesCtx, time.Second)
	defer cancel()
	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, expectedAuthFailureRegexes, crash.Timeout(60*time.Second))
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer func(ctx context.Context) {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Error("Couldn't clean up files: ", err)
		}
	}(removeFilesCtx)

	if len(files[crashAuthFailureMetaName]) == 1 {
		metaFile := files[crashAuthFailureMetaName][0]
		contents, err := ioutil.ReadFile(metaFile)
		if err != nil {
			s.Errorf("Couldn't read meta file %s contents: %v", metaFile, err)
		} else {
			if res, err := regexp.Match(sigAuthFailure, contents); err != nil {
				s.Error("Failed to run regex: ", err)
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			} else if !res {
				s.Error("Failed to find the expected auth_failure signature")
				if err := crash.MoveFilesToOut(ctx, s.OutDir(), metaFile); err != nil {
					s.Error("Failed to save the meta file: ", err)
				}
			}
		}
	} else {
		s.Errorf("Unexpectedly found multiple meta files: %q", files[crashAuthFailureMetaName])
		if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[crashAuthFailureMetaName]...); err != nil {
			s.Error("Failed to save unexpected crashes: ", err)
		}
	}
}
