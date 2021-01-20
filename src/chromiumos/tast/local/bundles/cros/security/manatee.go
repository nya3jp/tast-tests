// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	trichechusPath = "/usr/bin/trichechus"
	dugongPath     = "/usr/bin/dugong"
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

func getCallerLineNo() string {
	_, file, line, _ := runtime.Caller(1)
	return fmt.Sprintf("%s:%d", file, line)
}

func stopCmd(cmd testexec.Cmd) error {
	if err := cmd.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil && err.Error() != "signal: terminated" {
		return err
	}

	return nil
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
func (z *sireniaServices) Start(ctx context.Context, s *testing.State) (err error) {
	if z.trichechus != nil || z.dugong != nil {
		s.Fatal("only call start once")
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

		if err2 := z.stopTrichechus(s); err2 != nil {
			s.Error(err2)
		}
	}()

	line, err := z.trichechusStderr.ReadString('\n')
	if err != nil {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}

	if line == "Syslog exists.\n" || line == "Creating syslog.\n" {
		line, err = z.trichechusStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if strings.Contains(line, "starting trichechus:") {
		line, err = z.trichechusStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if strings.Contains(line, "Unable to start new process group:") {
		line, err = z.trichechusStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if !strings.Contains(line, "waiting for connection at: ip://127.0.0.1:") {
		return errors.New("failed to locate listening URI")
	}

	port, err := strconv.Atoi(line[strings.LastIndexByte(line, ':')+1 : len(line)-1])
	if err != nil {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}

	z.dugong = testexec.CommandContext(ctx, minijailPath, "-u", dugongUser, "--", dugongPath, "-U", fmt.Sprintf("ip://127.0.0.1:%d", port))
	stderr, err = z.dugong.StderrPipe()
	if err != nil {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}
	z.dugongStderr = bufio.NewReader(stderr)

	err = z.dugong.Start()
	if err != nil {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}
	defer func() {
		if err == nil {
			return
		}

		if err2 := z.stopDugong(s); err2 != nil {
			s.Error(err2)
		}
	}()

	line, err = z.dugongStderr.ReadString('\n')
	if err != nil {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}

	if strings.Contains(line, "Starting dugong:") {
		line, err = z.dugongStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if strings.HasSuffix(line, "Opening connection to trichechus\n") {
		line, err = z.dugongStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if strings.HasSuffix(line, "Starting rpc\n") {
		line, err = z.dugongStderr.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
		}
	}

	if !strings.HasSuffix(line, "Finished dbus setup, starting handler.\n") {
		return errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo())
	}

	return nil
}

func (z *sireniaServices) stopTrichechus(s *testing.State) error {
	if z.trichechus == nil {
		s.Fatal("call start before stop")
	}

	return stopCmd(*z.trichechus)
}

func (z *sireniaServices) stopDugong(s *testing.State) error {
	if z.dugong == nil {
		s.Fatal("call start before stop")
	}

	return stopCmd(*z.dugong)
}

// Stop tears down the test environment. Start() must be called first.
func (z *sireniaServices) Stop(s *testing.State) (err error) {
	// Stopping trichechus first might cause dugong to exit with an error.
	if err = z.stopDugong(s); err != nil {
		s.Error("Failed to stop dugong: ", err)
	}
	if err = z.stopTrichechus(s); err != nil {
		s.Error("Failed to stop trichechus: ", err)
	}
	return err
}

// Manatee implements the security.Manatee test.
func Manatee(ctx context.Context, s *testing.State) {
	testCase := s.Param().(manateeTestCase)

	d := newSireniaServices()
	if !testCase.useSystemServices {
		if err := d.Start(ctx, s); err != nil {
			s.Fatal("Failed to start sirenia services: ", err)
		}
		defer func() {
			if err := d.Stop(s); err != nil {
				s.Error("Failed to stop sirenia services: ", err)
			}
		}()
	}
}
