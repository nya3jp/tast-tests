// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.Autoupdate,
		Desc: "Fixture for autoupdate tests, ensures that the DUT stays on initially provisioned test image",
		Contacts: []string{
			"gabormada@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		Impl:            &autoupdateFixt{},
		SetUpTimeout:    2 * time.Minute,
		PostTestTimeout: 15 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
	})
}

const forceProvisionPath = "/mnt/stateful_partition/.force_provision"

type autoupdateFixt struct {
	originalVersion string
	builderPath     string
}

// FixtData is the data returned by SetUp and passed to tests.
type FixtData struct {
	paygen *Paygen
}

// Paygen implements the WithPaygen interface.
func (f FixtData) Paygen() *Paygen {
	return f.paygen
}

// SetUp stores the version information that should be restored between tests.
// It loads the Paygen information as it is needed in most of the autoupdate tests.
func (au *autoupdateFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	lsbContent := map[string]string{
		lsbrelease.Version:     "",
		lsbrelease.BuilderPath: "",
	}

	if err := FillFromLSBRelease(ctx, s.DUT(), s.RPCHint(), lsbContent); err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	// Original image version to compare it with the version after the update.
	au.originalVersion = lsbContent[lsbrelease.Version]
	// Builder path is used in selecting the update image.
	au.builderPath = lsbContent[lsbrelease.BuilderPath]

	paygen, err := LoadPaygenFromGS(ctx)
	if err != nil {
		s.Fatal("Failed to load paygen data: ", err)
	}

	// If this file is not removed by the end of the test suite, Test Library Service will restore the OS image.
	if _, err := s.DUT().Conn().CommandContext(ctx, "touch", forceProvisionPath).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create file to request forced provisioning: ", err)
	}

	return &FixtData{paygen}
}

// PostTest ensures that the DUT has the original OS image for the upcoming test.
// If there is a different image it tries to restore the original version
//  - with a rollback first
//  - by installing the the original image again.
// PostTest fails if it cannot make sure the DUT has the original image.
func (au *autoupdateFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {
	checkTimeout := 1 * time.Minute
	rollbackTimeout := 4 * time.Minute

	// Limit the timeout for the version check.
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	// Check the connection to the DUT.
	if out, err := s.DUT().Conn().CommandContext(checkCtx, "echo", "1").Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run command over SSH: ", err)
	} else if string(out) != "1\n" {
		s.Fatalf("Invalid output when running command over SSH: got %q; want %q", string(out), "1")
	}

	// Check the image version.
	if version, err := ImageVersion(checkCtx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to read DUT image version after the test: ", err)
	} else if version == au.originalVersion {
		s.Log("No change in the OS image version after the test")
		return // There is no need to restore the image version.
	} else {
		s.Logf("Image version was not restored to %s after the test, it remained %s", au.originalVersion, version)
	}

	// Restore original image version with rollback.
	// TODO(b/236588876): Add a check if the other partition contains the original version or a different one.
	// Limit the timeout for rollback.
	rollbackCtx, cancel := context.WithTimeout(ctx, rollbackTimeout)
	defer cancel()

	s.Log("Restoring the original image with rollback")
	if err := s.DUT().Conn().CommandContext(rollbackCtx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		s.Fatal("Failed to rollback the DUT: ", err)
	}

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the rollback")
	if err := s.DUT().Reboot(rollbackCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version.
	if version, err := ImageVersion(rollbackCtx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to read DUT image version after the rollback: ", err)
	} else if version == au.originalVersion {
		s.Log("No change in the OS image version after the test")
		return // There is no need to restore the image version.
	} else {
		s.Logf("Image version was not restored to %s after the test, it remained %s", au.originalVersion, version)
	}

	// Restore the DUT image with installation.
	s.Log("Installing the original image to the DUT")
	err := UpdateFromGS(ctx, s.DUT(), s.OutDir(), s.RPCHint(), au.builderPath)
	if err != nil {
		s.Fatalf("Failed to restore DUT image to %q from GS: %v", au.builderPath, err)
	}

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the installing the original image")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT after installing a new image: ", err)
	}

	// Check the image version.
	if version, err := ImageVersion(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to read DUT image version after the image installation: ", err)
	} else if version != au.originalVersion {
		s.Fatalf("Failed to install image version; got %s, want %s", version, au.originalVersion)
	}
}

// TearDown checks if the force provisioning step can be skipped.
func (au *autoupdateFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	// Check the image version.
	version, err := ImageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the rollback: ", err)
	}

	// If the force provision is not deleted, the Test Library Service will restore the OS image after the tests are complete.
	if version == au.originalVersion {
		// Force provisioning can be skipped, as the DUT has the expected OS image version.
		if _, err := s.DUT().Conn().CommandContext(ctx, "rm", forceProvisionPath).Output(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to remove forced provisioning file: ", err)
		}
	}
}

func (*autoupdateFixt) Reset(ctx context.Context) error                       { return nil }
func (*autoupdateFixt) PreTest(ctx context.Context, s *testing.FixtTestState) {}
