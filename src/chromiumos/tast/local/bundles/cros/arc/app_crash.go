// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
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
		Pre:          arc.Booted(),
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

type buildProp struct {
	device      string
	board       string
	cpuAbi      string
	fingerprint string
}

func getProp(ctx context.Context, a *arc.ARC, key string) (string, error) {
	val, err := a.GetProp(ctx, key)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get %s", key)
	}
	if val == "" {
		return "", errors.Errorf("%s is empty", key)
	}
	return val, err
}

func getBuildProp(ctx context.Context, a *arc.ARC) (*buildProp, error) {
	device, err := getProp(ctx, a, "ro.product.device")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device")
	}
	board, err := getProp(ctx, a, "ro.product.board")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get board")
	}
	cpuAbi, err := getProp(ctx, a, "ro.product.cpu.abi")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cpu_abi")
	}
	fingerprint, err := getProp(ctx, a, "ro.build.fingerprint")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get fingerprint")
	}

	return &buildProp{
		device:      device,
		board:       board,
		cpuAbi:      cpuAbi,
		fingerprint: fingerprint,
	}, nil
}

func validateBuildProp(ctx context.Context, meta string, bp *buildProp) (bool, error) {
	f, err := os.Open(meta)
	if err != nil {
		return false, errors.Wrap(err, "failed to open meta file")
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return false, errors.Wrap(err, "failed to read meta file")
	}

	lines := strings.Split(string(b), "\n")
	contains := func(x string) bool {
		for _, l := range lines {
			if x == l {
				return true
			}
		}
		testing.ContextLogf(ctx, "Missing %q", x)
		return false
	}

	return contains("upload_var_device="+bp.device) &&
		contains("upload_var_board="+bp.board) &&
		contains("upload_var_cpu_abi="+bp.cpuAbi) &&
		contains("upload_var_arc_version="+bp.fingerprint), nil
}

func AppCrash(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

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
	// Wait files like com_android_settings_foo_bar.20200420.204845.664107.log in crashDir
	base := strings.ReplaceAll(exampleApp, ".", "_") + `(?:_[[:alnum:]]+)*.\d{8}.\d{6}.\d+`
	metaFileName := base + crash.MetadataExt
	files, err := crash.WaitForCrashFiles(ctx, []string{crashDir}, nil, []string{
		base + crash.LogExt, metaFileName, base + crash.InfoExt,
	})
	if err != nil {
		s.Fatal("didn't find files: ", err)
	}
	defer crash.RemoveAllFiles(ctx, files)

	bp, err := getBuildProp(ctx, a)
	if err != nil {
		// Upload /system/build.prop to invetigate because getprop sometimes fails to get the device name
		// even though the device name should always exists.
		// See details in https://bugs.chromium.org/p/chromium/issues/detail?id=1039512#c16
		if err := a.PullFile(ctx, "/system/build.prop", filepath.Join(s.OutDir(), "build.prop")); err != nil {
			s.Error("Failed to get build.prop: ", err)
		}
		s.Fatal("Failed to get BuildProperty: ", err)
	}

	metaFiles := files[metaFileName]
	if len(metaFiles) > 1 {
		s.Errorf("Unexpectedly saw %d crashes of appcrash. Saving for debugging", len(metaFiles))
		crash.MoveFilesToOut(ctx, s.OutDir(), metaFiles...)
	}
	// WaitForCrashFiles guarantees that there will be a match for all regexes if it succeeds,
	// so this must exist.
	isValid, err := validateBuildProp(ctx, metaFiles[0], bp)
	if err != nil {
		s.Fatal("Failed to validate meta file: ", err)
	}
	if !isValid {
		s.Error("validateBuildProp failed. Saving meta file")
		crash.MoveFilesToOut(ctx, s.OutDir(), metaFiles[0])
	}
}
