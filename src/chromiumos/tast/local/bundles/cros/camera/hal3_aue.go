// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3AUE,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that camera HAL3 will still function after it's device auto-update-expiration date",
		Contacts:     []string{"yerlandinata@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

// How many years in the future to simulate time leap
const aueYearOffset = 12

func getLibraryPath(name string) (string, error) {
	libPathsUsrLocal, err := filepath.Glob("/usr/local/lib*/" + name)
	if err != nil {
		return "", err
	}

	libPathsUsr, err := filepath.Glob("/usr/lib*/" + name)
	if err != nil {
		return "", err
	}

	libPaths := append(libPathsUsr, libPathsUsrLocal...)

	if len(libPaths) == 0 {
		return "", errors.New("Cannot find " + name)
	}
	return libPaths[0], nil
}

// timeLeap will modify the cmd object so that
// the given system command will be executed with shifted perception of time by "timeDelta".
// Do not use "sudo" as command because it does not support LD_PRELOAD
func timeLeap(cmd *testexec.Cmd, timeDelta time.Duration) error {
	if cmd.Env == nil || len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}

	libPath, err := getLibraryPath("libfake_date_time.so")
	if err != nil {
		return err
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("LD_PRELOAD=%s", libPath))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SECONDS_OFFSET=%d", int64(timeDelta.Seconds())))

	return nil
}

func createTestProcessCmd(ctx context.Context) (*testexec.Cmd, error) {

	cmd := testexec.CommandContext(ctx, "cros_camera_algo")

	timeOffsetHour := 24 * 365 * aueYearOffset
	offset, _ := time.ParseDuration(fmt.Sprintf("%dh", timeOffsetHour))
	if err := timeLeap(cmd, offset); err != nil {
		return nil, err
	}
	uid, gid, err := hal3.ArcCameraUIDAndGID()
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}

	return cmd, nil
}

func setup(ctx context.Context) (func(context.Context, *testing.State), error) {

	crosCameraAlgoStartCmd, err := createTestProcessCmd(ctx)
	if err != nil {
		return func(ctx context.Context, s *testing.State) {}, err
	}

	crosCameraAlgoJobName := "cros-camera-algo"
	if err := upstart.StopJob(ctx, crosCameraAlgoJobName); err != nil {
		return nil, errors.New("Failed to stop job: " + crosCameraAlgoJobName)
	}

	tearDownFunc := func(ctx context.Context, s *testing.State) {

		if crosCameraAlgoStartCmd.Process != nil {
			if err := crosCameraAlgoStartCmd.Process.Kill(); err != nil {
				s.Error("Test failed: failed to stop test process")
			}
		}

		if err := upstart.EnsureJobRunning(ctx, crosCameraAlgoJobName); err != nil {
			s.Error("Test failed: job can't resume job: " + crosCameraAlgoJobName)
		}

	}

	if err := crosCameraAlgoStartCmd.Start(); err != nil {
		return tearDownFunc, err
	}

	testing.ContextLogf(
		ctx,
		"Running %s (env: %v); PID: %d",
		strings.Join(crosCameraAlgoStartCmd.Args, " "),
		crosCameraAlgoStartCmd.Env,
		crosCameraAlgoStartCmd.Process.Pid,
	)

	return tearDownFunc, nil
}

func testLibfakedatetime(ctx context.Context) error {
	testing.ContextLog(ctx, "testing the libfake_date_time")

	// Always guaranteed more than exactly one year
	timeOffsetHour := 24 * 370
	offset, _ := time.ParseDuration(fmt.Sprintf("%dh", timeOffsetHour))

	dateCmd := testexec.CommandContext(ctx, "date", "+%Y")
	if err := timeLeap(dateCmd, offset); err != nil {
		return err
	}
	out, err := dateCmd.Output()
	if err != nil {
		return err
	}
	actualYear, err := strconv.Atoi(strings.Trim(string(out), "\n "))
	if err != nil {
		return err
	}
	expectedYear := time.Now().Year() + 1
	// One year is not always 365 days, for our use case it is fine as long as the year can increment
	// "year" is used because it is the easiest to parse and test
	if actualYear < expectedYear {
		return errors.Errorf("Assert libfake_date_time to simulate year (at least) %d, got: %d", expectedYear, actualYear)
	}
	testing.ContextLog(ctx, "libfake_date_time tests passed")
	return nil
}

func HAL3AUE(ctx context.Context, s *testing.State) {

	if err := testLibfakedatetime(ctx); err != nil {
		s.Error("libfake_date_time tests failed: ", err)
		return
	}

	tearDown, setupErr := setup(ctx)
	if tearDown != nil {
		defer tearDown(ctx, s)
	}
	if setupErr != nil {
		s.Error("Test failed on setup: ", setupErr)
		return
	}

	if err := hal3.RunTest(ctx, hal3.AUETestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}

}
