// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/arccrash"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppCrash,
		Desc:         "Test handling of a local app crash",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Name:              "mock_consent",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               crash.MockConsent,
		}, {
			Name:              "real_consent",
			ExtraSoftwareDeps: []string{"android_p", "metrics_consent"},
			Val:               crash.RealConsent,
		}, {
			Name:              "vm_mock_consent",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               crash.MockConsent,
		}},
	})
}

func AppCrash(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	opt := crash.WithMockConsent()
	useConsent := s.Param().(crash.ConsentType)
	if useConsent == crash.RealConsent {
		opt = crash.WithConsent(cr)
	}

	if err := crash.SetUpCrashTest(ctx, opt); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	s.Log("Starting app")
	const exampleApp = "com.android.settings"
	if err := a.Command(ctx, "am", "start", "-W", exampleApp).Run(); err != nil {
		s.Fatal("Failed to run an app to be crashed: ", err)
	}

	s.Log("Making crash")
	if err := a.Command(ctx, "am", "crash", exampleApp).Run(); err != nil {
		s.Fatal("Failed to crash: ", err)
	}

	s.Log("Getting crash dir path")
	user := cr.User()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "/crash")

	s.Log("Waiting for crash files to become present")
	// Wait files like com_android_settings_foo_bar.20200420.204845.12345.664107.log in crashDir
	base := strings.ReplaceAll(exampleApp, ".", "_") + `(?:_[[:alnum:]]+)*.\d{8}\.\d{6}\.\d+\.\d+`
	metaFileName := base + crash.MetadataExt
	files, err := crash.WaitForCrashFiles(ctx, []string{crashDir}, []string{
		base + crash.LogExt, metaFileName, base + crash.InfoExt,
	})
	if err != nil {
		s.Fatal("Didn't find files: ", err)
	}
	defer crash.RemoveAllFiles(ctx, files)

	metaFiles := files[metaFileName]
	if len(metaFiles) > 1 {
		s.Errorf("Unexpectedly saw %d crashes of appcrash. Saving for debugging", len(metaFiles))
		crash.MoveFilesToOut(ctx, s.OutDir(), metaFiles...)
	}
	// WaitForCrashFiles guarantees that there will be a match for all regexes if it succeeds,
	// so this must exist.
	metaFile := metaFiles[0]

	s.Log("Validating the meta file")
	bp, err := arccrash.GetBuildProp(ctx, a)
	if err != nil {
		if err := arccrash.UploadSystemBuildProp(ctx, a, s.OutDir()); err != nil {
			s.Error("Failed to get build.prop: ", err)
		}
		s.Fatal("Failed to get BuildProperty: ", err)
	}
	isValid, err := arccrash.ValidateBuildProp(ctx, metaFile, bp)
	if err != nil {
		s.Fatal("Failed to validate meta file: ", err)
	}
	if !isValid {
		s.Error("validateBuildProp failed. Saving meta file")
		crash.MoveFilesToOut(ctx, s.OutDir(), metaFile)
	}
}
