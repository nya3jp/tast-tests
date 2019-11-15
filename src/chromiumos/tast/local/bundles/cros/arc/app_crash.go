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

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppCrash,
		Desc:         "Test handling of a local app crash",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome", "chrome_internal"},
		Pre:          arc.Booted(),
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

func validateBuildProp(meta string, bp *buildProp, s *testing.State) (bool, error) {
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
		return false
	}

	return contains("upload_var_device="+bp.device) &&
		contains("upload_var_board="+bp.board) &&
		contains("upload_var_cpu_abi="+bp.cpuAbi) &&
		contains("upload_var_arc_version="+bp.fingerprint), nil
}

func AppCrash(ctx context.Context, s *testing.State) {
	const (
		pkg = "com.android.settings"
		cls = ".Settings"
	)
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer localCrash.TearDownCrashTest()

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	s.Log("Getting preexisting crashes")
	user := cr.User()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "/crash")

	oldCrashes, err := crash.GetCrashes(crashDir)
	if err != nil {
		s.Fatal("Couldn't get preexisting crashes: ", err)
	}

	if err := a.Command(ctx, "am", "crash", pkg).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Couldn't kill app: %s", err)
	}

	s.Log("Waiting for crash files to become present")
	files, err := localCrash.WaitForCrashFiles(ctx, []string{crashDir}, oldCrashes, []string{
		`com_android_settings.\d{8}.\d{6}.\d+.log`,
		`com_android_settings.\d{8}.\d{6}.\d+.meta`,
	})
	if err != nil {
		s.Fatal("didn't find files: ", err)
	}
	defer func() {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				s.Errorf("Couldn't clean up %s: %v", f, err)
			}
		}
	}()

	bp, err := getBuildProp(ctx, a)
	if err != nil {
		s.Fatal("Failed to get BuildProperty: ", err)
	}

	for _, f := range files {
		if filepath.Ext(f) != ".meta" {
			continue
		}
		isValid, err := validateBuildProp(f, bp, s)
		if err != nil {
			s.Fatal("Failed to validate meta file: ", err)
		}
		if !isValid {
			s.Error("validateBuildProp failed")
		}
		break
	}
}
