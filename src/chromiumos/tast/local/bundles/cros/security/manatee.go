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
		Params: []testing.Param{
			{
				Name:              "real",
				Val:               manateeTestCase{useSystemServices: true},
				ExtraAttr:         []string{},
				ExtraSoftwareDeps: []string{"manatee"},
			},
			{
				Name:      "fake",
				Val:       manateeTestCase{useSystemServices: false},
				ExtraAttr: []string{},
				//ExtraSoftwareDeps: []string{"sirenia"},
			},
		},
	})
}

func getCallerLineNo() string {
	_, file, line, _ := runtime.Caller(1)
	return fmt.Sprintf("%s:%d", file, line)
}

func combineErrors(e ...error) error {
	s := make([]error, 0)
	for _, err := range e {
		if err != nil {
			s = append(s, err)
		}
	}

	if len(s) == 0 {
		return nil
	}

	if len(s) == 1 {
		return s[0]
	}

	r := make([]string, len(s))
	for i, err := range s {
		r[i] = fmt.Sprintf("%v", err)
	}
	return errors.Errorf("%s", strings.Join(r, "; "))
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
	ctx              context.Context
	trichechus       *testexec.Cmd
	trichechusStderr *bufio.Reader
	dugong           *testexec.Cmd
	dugongStderr     *bufio.Reader
}

// newSireniaServices constructs the an instance of sireniaServices.
func newSireniaServices(ctx context.Context) *sireniaServices {
	return &sireniaServices{ctx: ctx}
}

// Start brings up a test environment. This is needed on non-ManaTEE images with sirenia.
func (s *sireniaServices) Start() error {
	if s.trichechus != nil || s.dugong != nil {
		return errors.New("only call start once")
	}

	s.trichechus = testexec.CommandContext(s.ctx, trichechusPath, "-U", "ip://127.0.0.1:0")
	stderr, err := s.trichechus.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get trichechus stderr")
	}
	s.trichechusStderr = bufio.NewReader(stderr)

	err = s.trichechus.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start trichechus")
	}

	line, err := s.trichechusStderr.ReadString('\n')
	if err != nil {
		return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
	}

	if line == "Syslog exists.\n" || line == "Creating syslog.\n" {
		line, err = s.trichechusStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if strings.Contains(line, "starting trichechus:") {
		line, err = s.trichechusStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if strings.Contains(line, "Unable to start new process group:") {
		line, err = s.trichechusStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if !strings.Contains(line, "waiting for connection at: ip://127.0.0.1:") {
		return errors.New("failed to locate listening URI")
	}

	port, err := strconv.Atoi(line[strings.LastIndexByte(line, ':')+1 : len(line)-1])
	if err != nil {
		return combineErrors(errors.Wrapf(err, "failed to parse listening port, got %q", line), s.stopTrichechus())
	}

	s.dugong = testexec.CommandContext(s.ctx, minijailPath, "-u", dugongUser, "--", dugongPath, "-U", fmt.Sprintf("ip://127.0.0.1:%d", port))
	stderr, err = s.dugong.StderrPipe()
	if err != nil {
		return combineErrors(errors.Wrap(err, "failed to get dugong stderr"), s.stopTrichechus())
	}
	s.dugongStderr = bufio.NewReader(stderr)

	err = s.dugong.Start()
	if err != nil {
		return combineErrors(errors.Wrap(err, "failed start dugong"), s.stopTrichechus())
	}

	line, err = s.dugongStderr.ReadString('\n')
	if err != nil {
		return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
	}

	if strings.Contains(line, "Starting dugong:") {
		line, err = s.dugongStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if strings.HasSuffix(line, "Opening connection to trichechus\n") {
		line, err = s.dugongStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if strings.HasSuffix(line, "Starting rpc\n") {
		line, err = s.dugongStderr.ReadString('\n')
		if err != nil {
			return combineErrors(errors.Wrapf(err, "%s failed to read stderr", getCallerLineNo()), s.stopTrichechus())
		}
	}

	if !strings.HasSuffix(line, "Finished dbus setup, starting handler.\n") {
		return combineErrors(errors.Wrap(err, "dugong failed to setup D-Bus"), s.stopTrichechus())
	}

	return nil
}

func (s *sireniaServices) stopTrichechus() error {
	if s.trichechus == nil {
		return errors.New("call start before stop")
	}

	if err := stopCmd(*s.trichechus); err != nil {
		errors.Wrap(err, "failed to stop trichechus")
	}
	return nil
}

func (s *sireniaServices) stopDugong() error {
	if s.dugong == nil {
		return errors.New("call start before stop")
	}

	if err := stopCmd(*s.dugong); err != nil {
		errors.Wrap(err, "failed to stop dugong")
	}
	return nil
}

// Stop tears down the test environment. Start() must be called first.
func (s *sireniaServices) Stop() (err error) {
	// Stopping trichechus first might cause dugong to exit with an error.
	return combineErrors(s.stopDugong(), s.stopTrichechus())
}

// Manatee implements the security.Manatee test.
func Manatee(ctx context.Context, s *testing.State) {
	testCase := s.Param().(manateeTestCase)

	d := newSireniaServices(ctx)
	if !testCase.useSystemServices {
		if err := d.Start(); err != nil {
			s.Fatal("Failed to start sirenia services: ", err)
		}
		defer func() {
			if err := d.Stop(); err != nil {
				s.Error("Failed to stop sirenia services: ", err)
			}
		}()
	}
}
