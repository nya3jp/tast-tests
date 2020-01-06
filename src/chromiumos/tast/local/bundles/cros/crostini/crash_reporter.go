// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

const (
	expectedRegex = `.*\.log`
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrashReporter,
		Desc:         "Check that crashes inside the VM produce crash reports",
		Contacts:     []string{"sidereal@google.com", "mutexlox@google.com"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:      "artifact",
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name:    "download",
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "download_buster",
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
		},
	})
}

func CrashReporter(ctx context.Context, s *testing.State) {
	data := s.PreValue().(crostini.PreData)

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(data.Chrome)); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest()

	oldFiles, err := crash.GetCrashes(crash.SystemCrashDir)
	if err != nil {
		s.Fatal("Failed to get original crashes: ", err)
	}

	// Trigger a crash in the root namespace of the VM
	cmd := data.Container.VM.Command(ctx, "python3", "-c", "import os\nos.abort()")
	// Ignore errors as this command is supposed to crash
	_ = cmd.Run()
	s.Log("Triggered a crash in the VM")

	if _, err := crash.WaitForCrashFiles(ctx, []string{crash.UserCrashDir}, oldFiles, []string{expectedRegex}); err != nil {
		s.Error("Couldn't find expected files: ", err)
	}
}
