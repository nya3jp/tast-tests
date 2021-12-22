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
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IntegratedU2F,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify U2F using the on-board cr50 firmware works",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"chrome", "gsc", "reboot"},
		Attr:         []string{"group:firmware", "firmware_cr50"},
		VarDeps:      []string{"servo"},
		ServiceDeps: []string{
			"tast.cros.example.ChromeService",
			"tast.cros.hwsec.AttestationDBusService",
		},
		Timeout: 10 * time.Minute,
	})
}

// IntegratedU2F verifies U2F using the on-board cr50 firmware works
func IntegratedU2F(ctx context.Context, s *testing.State) {
	// Create hwsec helper.
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewFullHelper(cmdRunner, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create hwsec remote helper: ", err)
	}

	// Ensure TPM is ready before running the tests.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure TPM is ready: ", err)
	}

	// Connect to the chrome service server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// u2fd reads files from the user's home dir, so we need to log in.
	cr := example.NewChromeServiceClient(cl.Conn)
	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx, &empty.Empty{})

	// Ensure TPM is prepared for enrollment.
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	chaps, err := pkcs11.NewChaps(ctx, cmdRunner, helper.CryptohomeClient())
	if err != nil {
		s.Fatal("Failed to create chaps client: ", err)
	}

	// Ensure chaps finished the initialization.
	// U2F didn't depend on chaps, but chaps would block the TPM operations, and caused U2F timeout.
	if err := util.EnsureChapsSlotsInitialized(ctx, chaps); err != nil {
		s.Fatal("Failed to ensure chaps slots: ", err)
	}

	// Connect to servo.
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	// Clean up the flags in u2fd after the tests finished.
	defer util.SetU2fdFlags(ctx, helper, false, false, false)

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
			if err := util.SetU2fdFlags(ctx, helper, tc.u2f, tc.g2f, tc.userKeys); err != nil {
				s.Fatal("Failed to set u2fd flags: ", err)
			}
			device, err := u2fDevicePath(ctx, cmdRunner)
			if err != nil {
				s.Fatal("Failed to get u2f device path: ", err)
			}

			//  Wait for system become stable.
			testing.Sleep(ctx, 3*time.Second)

			if err := runU2Test(ctx, s.DUT(), device, svo); err != nil {
				s.Fatal("U2F test filed: ", err)
			}
		})
	}
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

// runU2Test runs the U2FTest with the U2F device.
func runU2Test(ctx context.Context, dut *dut.DUT, device string, svo *servo.Servo) (retErr error) {
	const (
		u2fTestPath = "/usr/local/bin/U2FTest"
		trigger     = "Touch device and hit enter."
	)
	cmd := dut.Conn().CommandContext(ctx, "stdbuf", "-o0", u2fTestPath, device)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdout pipe")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdin pipe")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start U2fTest")
	}
	defer func() {
		if err := cmd.Wait(); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to wait U2fTest: ", err)
			} else {
				retErr = errors.Wrap(err, "failed to wait U2fTest")
			}
		}
	}()

	// Create the scanner.
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
		if strings.Contains(line, trigger) {
			testing.ContextLog(ctx, "Clicking power key")
			if err := svo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				return errors.Wrap(err, "failed to press the power key")
			}
			if _, err := stdin.Write([]byte("\n")); err != nil {
				return errors.Wrap(err, "failed to pipe the enter")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan stdin")
	}
	return nil
}
