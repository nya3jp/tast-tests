// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IntegratedU2F,
		Desc: "Verify U2F using the on-board cr50 firmware works",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"chrome", "gsc", "reboot"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"servo"},
		ServiceDeps: []string{
			"tast.cros.example.ChromeService",
			"tast.cros.hwsec.AttestationDBusService",
		},
		Timeout: 10 * time.Minute,
	})
}

func IntegratedU2F(ctx context.Context, s *testing.State) {
	// Connect to servo.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	// Create hwsec helper.
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewFullHelper(cmdRunner, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Resets the TPM states before running the tests.
	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	// Connect to the chrome service server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := example.NewChromeServiceClient(cl.Conn)

	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx, &empty.Empty{})

	// Ensure TPM is prepared for enrollment
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	for _, tc := range []struct {
		name     string
		u2f      bool
		g2f      bool
		userKeys bool
	}{
		{
			name:     "u2f",
			u2f:      true,
			g2f:      false,
			userKeys: false,
		},
		{
			name:     "g2f",
			u2f:      false,
			g2f:      true,
			userKeys: false,
		},
		{
			name:     "u2f_user_keys",
			u2f:      true,
			g2f:      false,
			userKeys: true,
		},
		{
			name:     "g2f_user_keys",
			u2f:      false,
			g2f:      true,
			userKeys: true,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := setU2fdFlags(ctx, helper, tc.u2f, tc.g2f, tc.userKeys); err != nil {
				s.Fatal("Failed to set u2fd flags: ", err)
			}
			device, err := u2fDevicePath(ctx, cmdRunner)
			if err != nil {
				s.Fatal("Failed to get u2f device path: ", err)
			}
			if err := runU2Test(ctx, s.DUT(), device, svo); err != nil {
				s.Fatal("U2F test filed: ", err)
			}
		})
	}
}

// setU2fdFlags sets flags to u2fd
func setU2fdFlags(ctx context.Context, helper *hwsecremote.FullHelperRemote, u2f, g2f, userKeys bool) (retErr error) {
	const (
		uf2ForcePath      = "/var/lib/u2f/force/u2f.force"
		gf2ForcePath      = "/var/lib/u2f/force/g2f.force"
		userKeysForcePath = "/var/lib/u2f/force/user_keys.force"
	)

	cmd := helper.CmdRunner()
	dCtl := helper.DaemonController()

	if err := dCtl.Stop(ctx, hwsec.U2fdDaemon); err != nil {
		return errors.Wrap(err, "failed to stop u2fd")
	}
	defer func() {
		if err := dCtl.Start(ctx, hwsec.U2fdDaemon); err != nil {
			retErr = errors.Wrap(err, "failed to restart u2fd")
		}
	}()

	// Remove flags.
	if _, err := cmd.Run(ctx, "sh", "-c", "rm -f /var/lib/u2f/force/*.force"); err != nil {
		return errors.Wrap(err, "failed to remove flags")
	}
	if u2f {
		if _, err := cmd.Run(ctx, "touch", uf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set u2f flag")
		}
	}
	if g2f {
		if _, err := cmd.Run(ctx, "touch", gf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set g2f flag")
		}
	}
	if userKeys {
		if _, err := cmd.Run(ctx, "touch", userKeysForcePath); err != nil {
			return errors.Wrap(err, "failed to set userKeys flag")
		}
	}
	return retErr
}

// u2fDevicePath returns the integrated u2f device path.
func u2fDevicePath(ctx context.Context, cmd *hwsecremote.CmdRunnerRemote) (string, error) {
	const (
		VID = "18D1"
		PID = "502C"
	)

	lsCmd := fmt.Sprintf("ls /sys/bus/hid/devices/*:%s:%s.*/hidraw", VID, PID)
	var dev string
	err := testing.Poll(ctx, func(context.Context) error {
		data, err := cmd.Run(ctx, "sh", "-c", lsCmd)
		if err != nil {
			return errors.Wrap(err, "failed to list files")
		}
		dev = strings.TrimSpace(string(data))
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: time.Second,
	})

	if err != nil {
		return "", errors.Wrap(err, "failed to find hid device")
	}
	return "/dev/" + dev, nil
}

func runU2Test(ctx context.Context, dut *dut.DUT, device string, svo *servo.Servo) error {
	const (
		u2fTestPath = "/usr/local/bin/U2FTest"
		trigger     = "Touch device and hit enter."
	)
	testCmd := fmt.Sprintf("stdbuf -o0 %s %s", u2fTestPath, device)
	cmd := dut.Command("sh", "-c", testCmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdout pipe")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdin pipe")
	}

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start U2fTest")
	}

	scanner := bufio.NewScanner(stdout)
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		index := bytes.IndexAny(data, ".\n")
		if index != -1 {
			return index + 1, data[:index+1], nil
		}
		if atEOF {
			return len(data), nil, io.EOF
		}
		return 0, nil, nil
	}
	scanner.Split(split)

	for scanner.Scan() {
		line := scanner.Text()
		testing.ContextLog(ctx, line)
		if strings.Contains(line, trigger) {
			testing.ContextLog(ctx, "Clicking power key")

			testing.Sleep(ctx, time.Second)

			if err := svo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				return errors.Wrap(err, "failed to press the power key")
			}

			testing.Sleep(ctx, time.Second)

			if _, err := stdin.Write([]byte("\n")); err != nil {
				return errors.Wrap(err, "failed to pipe the enter")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan stdin")
	}
	if err := cmd.Wait(ctx); err != nil {
		return errors.Wrap(err, "failed to wait U2fTest")
	}
	return nil
}
