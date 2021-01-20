// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	trichechusName = "trichechus"
	dugongName     = "dugong"
	dugongUser     = "dugong"
	minijailPath   = "/sbin/minijail0"
)

type manateeTestCase struct {
	useSystemServices bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Manatee,
		Desc: "Checks basic functionality of core ManaTEE features",
		Contacts: []string{
			"allenwebb@chromium.org",
			"cros-manatee@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "real",
				Val:               manateeTestCase{useSystemServices: true},
				ExtraSoftwareDeps: []string{"manatee"},
			},
			{
				Name:              "fake",
				Val:               manateeTestCase{useSystemServices: false},
				ExtraSoftwareDeps: []string{"sirenia"},
			},
		},
	})
}

func stopCmd(cmd testexec.Cmd) error {
	// SIGKILL (sent by Cmd.Kill()) does not allow cleanup hooks to run. Upstart uses SIGTERM to notify daemons when
	// their job is being stopped, so it is used here.
	if err := cmd.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	// Signal would have failed above if the process already exited.
	err := cmd.Wait()
	status, ok := testexec.GetWaitStatus(err)

	// Handle the case the process catches the signal and returns 0.
	if ok {
		return nil
	}

	// Handle the case the process didn't catch the signal.
	if status.Signaled() && status.Signal() == syscall.SIGTERM {
		return nil
	}

	return err
}

// sireniaServices provides an interface to ManaTEE through Sirenia. It also handles bring-up and tear-down of a test
// sirenia environment on non-ManaTEE images.
type sireniaServices struct {
	trichechus       *testexec.Cmd
	trichechusStderr *bufio.Reader
	dugong           *testexec.Cmd
	dugongStderr     *bufio.Reader
}

// newSireniaServices constructs the an instance of sireniaServices.
func newSireniaServices() *sireniaServices {
	return &sireniaServices{}
}

// Start brings up a test environment. This is needed on non-ManaTEE images with sirenia.
func (z *sireniaServices) Start(ctx context.Context) (err error) {
	if z.trichechus != nil || z.dugong != nil {
		return errors.New("already initialized; only call start once")
	}

	trichechusPath, err := exec.LookPath(trichechusName)
	if err != nil {
		return errors.Wrap(err, "cannot find trichechus")
	}

	dugongPath, err := exec.LookPath(dugongName)
	if err != nil {
		return errors.Wrap(err, "cannot find dugong")
	}

	z.trichechus = testexec.CommandContext(ctx, trichechusPath, "-U", "ip://127.0.0.1:0")
	stderr, err := z.trichechus.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get trichechus stderr")
	}
	z.trichechusStderr = bufio.NewReader(stderr)

	err = z.trichechus.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start trichechus")
	}
	defer func() {
		if err == nil {
			return
		}

		if err2 := z.stopTrichechus(); err2 != nil {
			testing.ContextLog(ctx, "Failed to stop trichechus: ", err2)
		}
	}()

	line, err := z.trichechusStderr.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed initial read from trichechus stderr")
	}

	// Skip expected lines only so error messages are caught.
	for _, condition := range []func(string) bool{
		func(l string) bool {
			return l == "Syslog exists.\n" || l == "Creating syslog.\n"
		},
		func(l string) bool {
			return strings.Contains(l, "starting trichechus:")
		},
		func(l string) bool {
			return strings.Contains(l, "Unable to start new process group:")
		},
	} {
		if condition(line) {
			if line, err = z.trichechusStderr.ReadString('\n'); err != nil {
				return errors.Wrapf(err, "failed to read from trichechus stderr; last line: %q", strings.TrimSpace(line))
			}
		}
	}

	if !strings.Contains(line, "waiting for connection at: ip://127.0.0.1:") {
		return errors.Errorf("failed to locate listening URI; last line: %q", line)
	}

	port, err := strconv.Atoi(line[strings.LastIndexByte(line, ':')+1 : len(line)-1])
	if err != nil {
		return errors.Wrapf(err, "failed to parse port from line: %q", line)
	}

	z.dugong = testexec.CommandContext(ctx, minijailPath, "-u", dugongUser, "--", dugongPath, "-U", fmt.Sprintf("ip://127.0.0.1:%d", port))
	stderr, err = z.dugong.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get dugong stderr")
	}
	z.dugongStderr = bufio.NewReader(stderr)

	err = z.dugong.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start dugong")
	}
	defer func() {
		if err == nil {
			return
		}

		if err2 := z.stopDugong(); err2 != nil {
			testing.ContextLog(ctx, "Failed to stop dugong: ", err2)
		}
	}()

	line, err = z.dugongStderr.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed initial read from dugong stderr")
	}

	// Skip expected lines only so error messages are caught.
	for _, condition := range []func(string) bool{
		func(l string) bool {
			return strings.Contains(l, "Starting dugong:")
		},
		func(l string) bool {
			return strings.HasSuffix(l, "Opening connection to trichechus\n")
		},
		func(l string) bool {
			return strings.HasSuffix(l, "Starting rpc\n")
		},
	} {
		if condition(line) {
			if line, err = z.dugongStderr.ReadString('\n'); err != nil {
				return errors.Wrapf(err, "failed to read from dugong stderr; last line: %q", strings.TrimSpace(line))
			}
		}
	}

	if !strings.HasSuffix(line, "Finished dbus setup, starting handler.\n") {
		return errors.Wrapf(err, "dugong failed to setup D-Bus; last line: %q", line)
	}

	return nil
}

func (z *sireniaServices) stopTrichechus() error {
	if z.trichechus == nil {
		return errors.New("trichechus not initialized; call start before stop")
	}

	return stopCmd(*z.trichechus)
}

func (z *sireniaServices) stopDugong() error {
	if z.dugong == nil {
		return errors.New("dugong not initialized; call start before stop")
	}

	return stopCmd(*z.dugong)
}

// Stop tears down the test environment. Start() must be called first.
func (z *sireniaServices) Stop(ctx context.Context) {
	// Stopping trichechus first might cause dugong to exit with an error.
	if err := z.stopDugong(); err != nil {
		testing.ContextLog(ctx, "Failed to stop dugong: ", err)
	}
	if err := z.stopTrichechus(); err != nil {
		testing.ContextLog(ctx, "Failed to stop trichechus: ", err)
	}
}

// Manatee implements the security.Manatee test.
func Manatee(ctx context.Context, s *testing.State) {
	testCase := s.Param().(manateeTestCase)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d := newSireniaServices()
	if !testCase.useSystemServices {
		if err := d.Start(ctx); err != nil {
			s.Fatal("Failed to start sirenia services: ", err)
		}
		defer func() {
			d.Stop(cleanupCtx)
		}()
	}
}
