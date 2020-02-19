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

const crashingAPKName = "ArcAppCrashTest.apk"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppCrash,
		Desc:         "Test handling of a local app crash",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Data:         []string{crashingAPKName},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

type buildProp struct {
	device      string
	board       string
	cpuAbi      string
	fingerprint string
}

func getBuildProp(ctx context.Context, a *arc.ARC) (*buildProp, error) {
	device, err := a.GetProp(ctx, "ro.product.device")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device")
	}
	board, err := a.GetProp(ctx, "ro.product.board")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get board")
	}
	cpuAbi, err := a.GetProp(ctx, "ro.product.cpu.abi")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cpu_abi")
	}
	fingerprint, err := a.GetProp(ctx, "ro.build.fingerprint")
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
	const (
		pkg = "org.chromium.arc.testapp.appcrash"
		cls = ".MainActivity"
	)
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer crash.TearDownCrashTest()

	// TODO(kansho): Use 'am crash' instead of the crashing app after all
	// Android N devices are gone.
	// The app was introduced because Android N doesn't support 'am crash'.
	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(crashingAPKName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// The app will crash by itself right after it starts.
	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed to run a crashing app: ", err)
	}

	s.Log("Getting crash dir path")
	user := cr.User()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "/crash")

	s.Log("Waiting for crash files to become present")
	const base = `org_chromium_arc_testapp_appcrash.\d{8}.\d{6}.\d+`
	const metaFileName = base + crash.MetadataExt
	files, err := crash.WaitForCrashFiles(ctx, []string{crashDir}, nil, []string{
		base + crash.LogExt, metaFileName, base + crash.InfoExt,
	})
	if err != nil {
		s.Fatal("didn't find files: ", err)
	}
	defer crash.RemoveAllFiles(ctx, files)

	bp, err := getBuildProp(ctx, a)
	if err != nil {
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
