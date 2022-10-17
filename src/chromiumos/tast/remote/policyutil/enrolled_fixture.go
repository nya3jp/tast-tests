// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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
		SetUpTimeout:    15 * time.Minute,
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
	if !s.DUT().Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	vpdOK := true
	if err := checkVPDState(ctx, s.DUT()); err != nil {
		// TODO(b/253326688): Change Log to Error when VPD restoration works again.
		vpdOK = false
		s.Log("VPD broken, trying to enroll anyway: ", err)
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

	// TODO(crbug.com/1187473): use a temporary directory.
	e.fdmsDir = fakedms.EnrollmentFakeDMSDir

	// Collect errors of enrollment attempts and raise them even in case of a Fatal error.
	var errs []error
	defer func() {
		for _, err := range errs {
			if !ok {
				s.Error("Failed to enroll: ", err)
			} else {
				s.Log("Failed enrolling attempt: ", err)
			}
		}
	}()

	for tries := 1; tries < 5; tries++ {
		// Make sure we have enough time to perform enrollment.
		// This helps differentiate real issues from timeout hitting different components.
		if deadline, ok := ctx.Deadline(); !ok {
			s.Fatal("missing deadline for context: ", ctx)
		} else if diff := deadline.Sub(time.Now()); diff < enrollmentRunTimeout {
			s.Fatalf("not enought time to perform setup and enrollment: have %s; need %s", diff, enrollmentRunTimeout)
		}

		s.Logf("Attempting enrollment, try %d", tries)
		attemptDir := path.Join(s.OutDir(), fmt.Sprintf("Attempt_%d", tries))

		enrollCtx, cancel := context.WithTimeout(ctx, enrollmentRunTimeout)
		defer cancel()

		if err := EnsureTPMAndSystemStateAreReset(enrollCtx, s.DUT(), s.RPCHint()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}

		// Check connection state after reboot.
		if !s.DUT().Connected(ctx) {
			s.Fatal("Failed DUT connection check after reboot")
		}

		if err := enroll(enrollCtx, attemptDir, s.DUT(), s.RPCHint(), e.fdmsDir, vpdOK); err != nil {
			errs = append(errs, err)
		} else {
			// When the enrollment is successful, there is no need to retry again.
			break
		}
	}

	if len(errs) == 0 {
		ok = true
	}

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

func enroll(ctx context.Context, attemptDir string, dut *dut.DUT, rpcHint *testing.RPCHint, fdmsDir string, vpdOK bool) (retErr error) {
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	policyClient := pspb.NewPolicyServiceClient(cl.Conn)

	if _, err := policyClient.CreateFakeDMSDir(ctx, &pspb.CreateFakeDMSDirRequest{
		Path: fdmsDir,
	}); err != nil {
		return errors.Wrap(err, "failed to create FakeDMS directory")
	}

	ok := false
	defer func() {
		if !ok {
			if _, err := policyClient.RemoveFakeDMSDir(ctx, &pspb.RemoveFakeDMSDirRequest{
				Path: fdmsDir,
			}); err != nil {
				if retErr != nil {
					retErr = errors.Wrap(err, "failed to remove FakeDMS directory")
				} else {
					testing.ContextLog(ctx, "Failed to remove FakeDMS directory: ", err)
				}
			}
		}
	}()

	// Always dump the logs.
	defer func(ctx context.Context) {
		if err := os.Mkdir(attemptDir, 0777); err != nil {
			testing.ContextLog(ctx, "Failed to create attempt dir: ", err)
		}

		chromeDir := path.Join(attemptDir, "Chrome")
		if err := os.Mkdir(chromeDir, 0777); err != nil {
			testing.ContextLog(ctx, "Failed to create Chrome dir: ", err)
		}

		if err := linuxssh.GetFile(ctx, dut.Conn(), "/var/log/chrome/chrome", filepath.Join(chromeDir, "chrome.log"), linuxssh.DereferenceSymlinks); err != nil {
			testing.ContextLog(ctx, "Failed to dump Chrome log: ", err)
		}

		fdmsDirHost := path.Join(attemptDir, "FakeDMS")
		if err := os.Mkdir(fdmsDirHost, 0777); err != nil {
			testing.ContextLog(ctx, "Failed to create FakeDMS dir: ", err)
		}

		if err := linuxssh.GetFile(ctx, dut.Conn(), fdmsDir, fdmsDirHost, linuxssh.DereferenceSymlinks); err != nil {
			testing.ContextLog(ctx, "Failed to dump FakeDMS dir: ", err)
		}
	}(ctx)

	pJSON, err := json.Marshal(policy.NewBlob())
	if err != nil {
		return errors.Wrap(err, "failed to marshal policy blob")
	}

	if _, err := policyClient.EnrollUsingChrome(ctx, &pspb.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		FakedmsDir: fdmsDir,
		SkipLogin:  true,
	}); err != nil {
		if !vpdOK {
			return errors.Wrap(err, "VPD broken, likely cause of enrollment failure")
		}

		return errors.Wrap(err, "failed to enroll using Chrome")
	}

	if _, err := policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to stop Chrome and FakeDMS")
	}

	ok = true

	return nil
}
