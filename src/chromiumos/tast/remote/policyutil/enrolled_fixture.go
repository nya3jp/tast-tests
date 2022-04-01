// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"encoding/json"
	"io/ioutil"
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
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.Enrolled,
		Desc:            "Fixture providing enrollment",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &enrolledFixt{},
		SetUpTimeout:    8 * time.Minute,
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
	if err := checkVPDState(ctx, s.DUT()); err != nil {
		s.Fatal("VPD broken, skipping enrollment: ", err)
	}

	if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
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

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	// TODO(crbug.com/1187473): use a temporary directory.
	e.fdmsDir = fakedms.EnrollmentFakeDMSDir

	if _, err := pc.CreateFakeDMSDir(ctx, &ps.CreateFakeDMSDirRequest{
		Path: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to create temporary directory for FakeDMS: ", err)
	}

	// Always dump the logs.
	defer func() {
		if err := linuxssh.GetFile(ctx, s.DUT().Conn(), fakedms.EnrollmentFakeDMSDir, filepath.Join(s.OutDir(), "EnrollmentFakeDMSDir"), linuxssh.PreserveSymlinks); err != nil {
			s.Log("Failed to dump: ", err)
		}
	}()

	pJSON, err := json.Marshal(policy.NewBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		FakedmsDir: e.fdmsDir,
		SkipLogin:  true,
	}); err != nil {
		s.Fatal("Failed to enroll using Chrome: ", err)
	}

	if _, err := pc.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to stop Chrome: ", err)
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

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.RemoveFakeDMSDir(ctx, &ps.RemoveFakeDMSDirRequest{
		Path: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to remove temporary directory for FakeDMS: ", err)
	}
}

func (*enrolledFixt) Reset(ctx context.Context) error                        { return nil }
func (*enrolledFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*enrolledFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
