// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"

	pb "chromiumos/tast/services/cros/firmware"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateKernelVersion,
		Desc:         "Update kernel version bits in CGPT and verify its consistency",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.firmware.CgptService"},
		Fixture:      fixture.DevMode,
	})
}

func kernelImageVersion(ctx context.Context, h *firmware.Helper, imagePath string) (string, error) {
	vbutilOutput, err := h.DUT.Conn().CommandContext(ctx, "vbutil_kernel", "--verify", imagePath).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to call vbutil_kernel")
	}

	for _, line := range strings.Split(string(vbutilOutput), "\n") {
		if strings.Contains(line, "Kernel version:") {
			if len(strings.Fields(line)) > 2 {
				return "", errors.Wrapf(err, "Failed to determine kernel version from vbutil_kernel output")
			}
			return strings.Fields(line)[2], nil

		}
	}
	return "", nil
}

func rebootDUT(ctx context.Context, h *firmware.Helper, s *testing.State) error {
	h.CloseRPCConnection(ctx)
	if err := h.DUT.Reboot(ctx); err != nil {
		return errors.Wrapf(err, "Failed to reboot DUT")
	}

	s.Log("Wait for DUT to reconnect")
	if err := h.DUT.WaitConnect(ctx); err != nil {
		return errors.Wrapf(err, "Failed to reconnect to DUT")
	}

	s.Log("Reconnecting to RPC services on DUT")
	if err := h.RequireRPCClient(ctx); err != nil {
		return errors.Wrapf(err, "Failed to reconnect to the RPC service on DUT")
	}

	s.Log("Reconnecting to CgptService on DUT")
	if err := h.RequireCgptServiceClient(ctx); err != nil {
		return errors.Wrapf(err, "Failed to reconnect to BiosServiceClient on DUT")
	}

	if err := h.EnsureDUTBooted(ctx); err != nil {
		return errors.Wrapf(err, "Failed to ensure the DUT is booted!")
	}

	return nil
}

func setKernelImageVersion(ctx context.Context, h *firmware.Helper, s *testing.State, imagePath, version string) error {
	const (
		kernelPrivateKeyPath = "/usr/share/vboot/devkeys/kernel_data_key.vbprivk"
		kernelKeyblockPath   = "/usr/share/vboot/devkeys/kernel.keyblock"
	)

	s.Logf("Repacking kernel image %s with version %s to tmpfs", imagePath, version)
	vbutilOutput, err := h.DUT.Conn().CommandContext(ctx,
		"vbutil_kernel", "--repack",
		"/tmp/kernel-repack.bin",
		"--oldblob", imagePath,
		"--signprivate", kernelPrivateKeyPath,
		"--keyblock", kernelKeyblockPath,
		"--version", version,
	).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "Failed to load repack kernel from image %s with version %s: %s", imagePath, version, string(vbutilOutput))
	}

	s.Log("Writing kernel image back...")
	ddOutput, err := h.DUT.Conn().CommandContext(ctx,
		"dd", "if=/tmp/kernel-repack.bin", "of="+imagePath, "conv=sync",
	).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "Failed to write patched kernel image back to %s with version %s: %s", imagePath, version, string(ddOutput))
	}
	return nil
}

// UpdateKernelVersion reads CGPT headers of kernel partition and
// then modifies the version bits to mimic kernel update and checks
// if it persist after reboot
func UpdateKernelVersion(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	s.Log("Acquiring CgptService")
	if err := h.RequireCgptServiceClient(ctx); err != nil {
		s.Fatal("Requiring CgptServiceClient: ", err)
	}

	s.Log("Getting root device")
	rootDev, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s", "-d").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to acquire current root device: ", err)
	}
	rootDev = []byte(strings.TrimSuffix(string(rootDev), "\n"))
	s.Logf("Root device is %s", rootDev)

	s.Log("Reading CGPT table")
	originalCgptTable, err := h.CgptServiceClient.GetCgptTable(ctx, &pb.GetCgptTableRequest{
		BlockDevice: string(rootDev),
	})
	if err != nil {
		s.Fatalf("Failed to acquire CGPT table for root device %s: %s", rootDev, err)
	}

	kernApath := ""
	for _, part := range originalCgptTable.CgptTable {
		if part.Label == "KERN-A" {
			kernApath = part.PartitionPath
			s.Log("KERN-A partition is: ", kernApath)
			break
		}
	}
	if kernApath == "" {
		s.Fatal("Failed to find KERN-A partition, check your DUT integrity")
	}

	oldKernelVersion, err := kernelImageVersion(ctx, h, kernApath)
	if err != nil {
		s.Fatalf("Failed to determine kernel version for %s: %w", kernApath, err)
	}

	defer func() {
		s.Log("Restoring original CGPT table")
		if _, err := h.CgptServiceClient.RestoreCgptAttributes(ctx, &pb.RestoreCgptAttributesRequest{
			CgptTable:   originalCgptTable.CgptTable,
			BlockDevice: string(rootDev),
		}); err != nil {
			s.Fatal("Failed to restore CGPT table: ", err)
		}

		s.Log("Restoring original kernel version")
		if err := setKernelImageVersion(ctx, h, s, kernApath, oldKernelVersion); err != nil {
			s.Fatalf("Failed to set kernel version for %s: %w", kernApath, err)
		}
	}()

	if err := setKernelImageVersion(ctx, h, s, kernApath, "2"); err != nil {
		s.Fatalf("Failed to set kernel version for %s: %w", kernApath, err)
	}

	s.Log("Rebooting DUT")
	if err := rebootDUT(ctx, h, s); err != nil {
		s.Fatal("Unable to return DUT: ", err)
	}

	s.Log("Reading new CGPT table")
	newCgptTable, err := h.CgptServiceClient.GetCgptTable(ctx, &pb.GetCgptTableRequest{
		BlockDevice: string(rootDev),
	})
	if err != nil {
		s.Fatalf("Failed to acquire CGPT table for root device %s: %s", rootDev, err)
	}
	successful := false
	for _, part := range newCgptTable.CgptTable {
		if part.Label == "KERN-A" {
			for _, attr := range part.Attrs {
				if attr.Name == "successful" && attr.Value == 1 {
					successful = true
					break
				}
			}
		}
	}
	if !successful {
		s.Fatal("KERN-A did not boot successfully")
	}

	s.Log("Reading new kernel version")
	newKernelVersion, err := kernelImageVersion(ctx, h, kernApath)
	if err != nil {
		s.Fatalf("Failed to determine kernel version for %s: %w", kernApath, err)
	}

	if oldKernelVersion == newKernelVersion {
		s.Fatalf("New kernel version %s wasn't set successfully", newKernelVersion)
	}

}
