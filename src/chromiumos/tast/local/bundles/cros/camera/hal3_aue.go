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

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
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
		Desc:         "Verifies that camera HAL3 will still function after its device auto-update-expiration date",
		Contacts:     []string{"yerlandinata@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc", "arc_camera3", "chrome", caps.BuiltinCamera},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

// aueYearOffset value will be added to current system date as the date of the
// simulation, assuming there are 86400 seconds in one day and 365 days in one year.
const aueYearOffset = 12
const secondsInOneDay = 86400
const daysInOneYear = 365

// libraryPath tries to find the a file that matches the name
// in common library path and then return the absolute path of it.
// If there are more than one, only one of it will be returned.
func libraryPath(name string) (string, error) {
	for _, path := range []string{"/usr/lib*/", "/usr/local/lib*/"} {
		libs, err := filepath.Glob(filepath.Join(path, name))
		if err != nil {
			return "", err
		}
		if len(libs) > 0 {
			return libs[0], nil
		}
	}
	return "", errors.New("cannot find " + name)
}

// timeLeap will modify the cmd object so that the given system command will
// be executed with shifted perception of time by the given seconds value.
// Do not use "sudo" as command, because it does not support LD_PRELOAD.
func timeLeap(cmd *testexec.Cmd, seconds int64) error {
	libPath, err := libraryPath("libfake_date_time.so")
	if err != nil {
		return err
	}

	if cmd.Env == nil || len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}

	cmd.Env = append(cmd.Env, "LD_PRELOAD="+libPath,
		fmt.Sprintf("SECONDS_OFFSET=%d", seconds))

	return nil
}

// createTestProcessCmd will return the cmd object of cros_camera_algo process,
// that is already configured for the test.
func createTestProcessCmd(ctx context.Context) (*testexec.Cmd, error) {
	cmd := testexec.CommandContext(ctx, "cros_camera_algo")
	const timeOffsetSeconds = secondsInOneDay * daysInOneYear * aueYearOffset
	if err := timeLeap(cmd, timeOffsetSeconds); err != nil {
		return nil, err
	}
	uid, gid, err := hal3.ArcCameraUIDAndGID()
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}

	return cmd, nil
}

// setup will prepare the test environment and services
// and will also return cleanup function.
func setup(ctx context.Context) (action.Action, error) {
	cleanup := func(ctx context.Context) error { return nil }
	crosCameraAlgoStartCmd, err := createTestProcessCmd(ctx)
	if err != nil {
		return cleanup, err
	}

	const crosCameraAlgoJobName = "cros-camera-algo"
	if err := upstart.StopJob(ctx, crosCameraAlgoJobName); err != nil {
		return cleanup, errors.Wrapf(err, "failed to stop job %q", crosCameraAlgoJobName)
	}

	cleanup = func(ctx context.Context) error {
		if crosCameraAlgoStartCmd.Process != nil {
			if err := crosCameraAlgoStartCmd.Process.Kill(); err != nil {
				return errors.Wrap(err, "failed to stop test process")
			}
		}

		if err := upstart.EnsureJobRunning(ctx, crosCameraAlgoJobName); err != nil {
			return errors.Wrapf(err, "failed to resume job %s", crosCameraAlgoJobName)
		}

		return nil
	}

	if err := crosCameraAlgoStartCmd.Start(); err != nil {
		return cleanup, err
	}

	testing.ContextLogf(
		ctx,
		"Running %s (env: %v); PID: %d",
		strings.Join(crosCameraAlgoStartCmd.Args, " "),
		crosCameraAlgoStartCmd.Env,
		crosCameraAlgoStartCmd.Process.Pid,
	)

	return cleanup, nil
}

// testLibfakedatetime will assert the "future time simulation"
// functionality of libfake_date_time.
func testLibfakedatetime(ctx context.Context) error {
	testing.ContextLog(ctx, "Testing libfake_date_time")

	// Always guaranteed more than exactly one year
	const timeOffsetSeconds = secondsInOneDay * (daysInOneYear + 5)
	dateCmd := testexec.CommandContext(ctx, "date", "+%Y")
	if err := timeLeap(dateCmd, timeOffsetSeconds); err != nil {
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
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := testLibfakedatetime(ctx); err != nil {
		s.Fatal("libfake_date_time tests failed: ", err)
	}

	cleanup, err := setup(ctx)
	defer func(ctx context.Context) {
		if err := cleanup(ctx); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}(cleanupCtx)

	if err != nil {
		s.Fatal("Test failed on setup: ", err)
	}

	if err := hal3.RunTest(ctx, hal3.AUETestConfig()); err != nil {
		s.Error("Test failed: ", err)
	}
}
