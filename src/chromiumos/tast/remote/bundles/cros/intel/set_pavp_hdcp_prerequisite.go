// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/remote/bundles/cros/intel/hdcpsetup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetPAVPHDCPPrerequisite,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies setting configuration flags before starting PAVP/HDCP tests",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"reboot", "chrome"},
		HardwareDeps: hwdep.D(hdcpsetup.PerfHDCPDevices()),
	})
}

// SetPAVPHDCPPrerequisite adds config flags that are required to
// set on DUT before executing PAVP/HDCP tests.
// PAVP stands for Protected Audio Video Path.
// HDCP stands for High-bandwidth Digital Content Protection.
func SetPAVPHDCPPrerequisite(ctx context.Context, s *testing.State) {
	dut := s.DUT()

	const (
		rootfsCmd         = "/usr/share/vboot/bin/make_dev_ssd.sh --remove_rootfs_verification --partitions 2"
		remount           = "sudo mount -o rw,remount /"
		devMode           = "--allow-ra-in-dev-mode "
		enableHevc        = "--enable-clear-hevc-for-testing"
		configFile        = "/etc/chrome_dev.conf"
		oemcryptoPath     = "/var/lib/oemcrypto"
		oemPublicCertFile = "oem_public_cert.bin"
		wrappedRSAKeyFile = "wrapped_rsa_key.bin"
		wrappedKeyboxFile = "wrapped_wv_keybox"
	)

	if err := dut.Conn().CommandContext(ctx, "bash", "-c", rootfsCmd).Run(); err != nil {
		s.Fatal("Failed to run remove rootfs command: ", err)
	}

	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	if err := dut.Conn().CommandContext(ctx, "bash", "-c", remount).Run(); err != nil {
		s.Fatal("Failed to run remount command: ", err)
	}

	out, err := dut.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("cat %s", configFile)).Output()
	if err != nil {
		s.Fatal("Failed to run cat config file command: ", err)
	}
	if !(strings.Contains(string(out), devMode) && strings.Contains(string(out), enableHevc)) {
		for _, text := range []string{devMode, enableHevc} {
			if err := dut.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("echo '%s' >> %s", text, configFile)).Run(); err != nil {
				s.Fatalf("Failed to write %s to %s: %v", text, configFile, err)
			}
		}
	}

	output, err := dut.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("ls %s", oemcryptoPath)).Output()
	if err != nil {
		s.Fatal("Failed to run oemcrypto command: ", err)
	}

	var oemCryptoContents = []string{oemPublicCertFile, wrappedRSAKeyFile, wrappedKeyboxFile}
	for _, val := range oemCryptoContents {
		if !strings.Contains(string(output), val) {
			s.Errorf("Failed to find %s at %s", val, oemcryptoPath)
		}
	}
}
