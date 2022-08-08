// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const enrollmentRunTimeout = 8 * time.Minute

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.Enrolled,
		Desc:            "Fixture providing enrollment",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &enrolledFixt{},
		SetUpTimeout:    3 * enrollmentRunTimeout,
		TearDownTimeout: 5 * time.Minute,
		ResetTimeout:    15 * time.Second,
		ServiceDeps:     []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService"},
	})
}

type enrolledFixt struct {
	fdmsDir string
}

func dumpVPDContent(ctx context.Context, d *dut.DUT) ([]byte, error) {
	out, err := d.Conn().CommandContext(ctx, "vpd", "-i", "RW_VPD", "-l").Output(ssh.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run the vpd command")
	}

	return out, nil
}

func checkVPDState(ctx context.Context, d *dut.DUT) error {
	// https://chromeos.google.com/partner/dlm/docs/factory/vpd.html#required-rw-fields
	const requiredField = "gbind_attribute"

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("no output directory")
	}

	if out, err := dumpVPDContent(ctx, d); err != nil {
		return err
	} else if !strings.Contains(string(out), requiredField) {
		// VPD is not running well, returning an error. Second run will confirm
		// whether the error is transitory.

		if err := ioutil.WriteFile(filepath.Join(outDir, "vpd-dump.txt"), out, 0644); err != nil {
			return errors.New("failed to dump VPD content")
		}

		out, err := dumpVPDContent(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed the second dump of the VPD")
		}
		if err := ioutil.WriteFile(filepath.Join(outDir, "vpd-dump-2.txt"), out, 0644); err != nil {
			return errors.New("failed to dump VPD content")
		}
		if strings.Contains(string(out), requiredField) {
			return errors.Errorf("VPD error, first run did not find %q, second run did", requiredField)
		}

		return errors.Errorf("VPD error, did not find the required field %q", requiredField)
	}

	return nil
}

func (e *enrolledFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if out, err := s.DUT().Conn().CommandContext(ctx, "echo", "1").Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run command over SSH: ", err)
	} else if string(out) != "1\n" {
		s.Fatalf("Invalid output when running command over SSH: got %q; want %q", string(out), "1")
	}

	vpdError := checkVPDState(ctx, s.DUT())
	if vpdError != nil {
		s.Log("VPD broken, trying to enroll anyway: ", vpdError)
	}

	ok := false
	defer func() {
		if !ok {
			s.Log("Removing enrollment after failing SetUp")
			if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}
		}
	}()

	tries := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Make sure we have enough time to perform enrollment.
		// This helps differentiate real issues from timeout hitting different components.
		if deadline, ok := ctx.Deadline(); !ok {
			return testing.PollBreak(errors.Errorf("missing deadline for context %v", ctx))
		} else if diff := deadline.Sub(time.Now()); diff < enrollmentRunTimeout {
			return testing.PollBreak(errors.New("not enought time to perform setup and enrollment"))
		}

		if out, err := s.DUT().Conn().CommandContext(ctx, "echo", "1").Output(ssh.DumpLogOnError); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run connect over SSH"))
		} else if string(out) != "1\n" {
			return testing.PollBreak(errors.Errorf("invalid output when running command over SSH: got %q; want %q", string(out), "1"))
		}

		tries = tries + 1
		s.Logf("Attempting enrollment, try %d", tries)

		ctx, cancel := context.WithTimeout(ctx, enrollmentRunTimeout)
		defer cancel()

		if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			return errors.Wrap(err, "failed to reset TPM")
		}

		// Check connection state after reboot.
		if out, err := s.DUT().Conn().CommandContext(ctx, "echo", "1").Output(ssh.DumpLogOnError); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run connect over SSH"))
		} else if string(out) != "1\n" {
			return testing.PollBreak(errors.Errorf("invalid output when running command over SSH: got %q; want %q", string(out), "1"))
		}

		// TODO(crbug.com/1187473): use a temporary directory.
		e.fdmsDir = fakedms.EnrollmentFakeDMSDir

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		policyClient := pspb.NewPolicyServiceClient(cl.Conn)

		if _, err := policyClient.CreateFakeDMSDir(ctx, &pspb.CreateFakeDMSDirRequest{
			Path: e.fdmsDir,
		}); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to create FakeDMS directory"))
		}

		enrollOK := false

		defer func() {
			if !enrollOK {
				if _, err := policyClient.RemoveFakeDMSDir(ctx, &pspb.RemoveFakeDMSDirRequest{
					Path: e.fdmsDir,
				}); err != nil {
					if ctx.Err() != nil {
						s.Error("Failed to remove FakeDMS directory: ", err)
					} else {
						s.Log("Failed to remove FakeDMS directory: ", err)
					}
				}
			}
		}()

		// Always dump the logs.
		defer func(ctx context.Context) {
			triesStr := strconv.Itoa(tries)

			attemptDir := path.Join(s.OutDir(), "Attempt_"+triesStr)

			if err := os.Mkdir(attemptDir, 0777); err != nil {
				s.Log("Failed to create attempt dir: ", err)
			}

			chromeDir := path.Join(attemptDir, "Chrome")
			if err := os.Mkdir(chromeDir, 0777); err != nil {
				s.Log("Failed to create Chrome dir: ", err)
			}

			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), "/var/log/chrome/chrome", filepath.Join(chromeDir, "chrome.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to dump Chrome log: ", err)
			}

			fdmsDir := path.Join(attemptDir, "FakeDMS")
			if err := os.Mkdir(fdmsDir, 0777); err != nil {
				s.Log("Failed to create FakeDMS dir: ", err)
			}

			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), e.fdmsDir, fdmsDir, linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to dump FakeDMS dir: ", err)
			}
		}(ctx)

		pJSON, err := json.Marshal(policy.NewBlob())
		if err != nil {
			return testing.PollBreak(err)
		}

		if _, err := policyClient.EnrollUsingChrome(ctx, &pspb.EnrollUsingChromeRequest{
			PolicyJson: pJSON,
			FakedmsDir: e.fdmsDir,
			SkipLogin:  true,
		}); err != nil {
			if vpdError != nil {
				return testing.PollBreak(errors.Wrap(err, "VPD broken, likely cause of enrollment failure"))
			}

			return errors.Wrap(err, "failed to enroll using Chrome")
		}

		if _, err := policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to stop Chrome and FakeDMS")
		}

		enrollOK = true

		return nil
	}, &testing.PollOptions{Timeout: 3*enrollmentRunTimeout + 15*time.Second}); err != nil {
		s.Fatal("Failed to enroll with retries: ", err)
	}

	ok = true

	return nil
}

func (e *enrolledFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := pspb.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.RemoveFakeDMSDir(ctx, &pspb.RemoveFakeDMSDirRequest{
		Path: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to remove temporary directory for FakeDMS: ", err)
	}
}

func (*enrolledFixt) Reset(ctx context.Context) error                        { return nil }
func (*enrolledFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*enrolledFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
