// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/remote/sysutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:         "rootfsRemoved",
		Desc:         "Removes rootfs verifications and reboots the DUT",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Impl:         &impl{},
		SetUpTimeout: 5 * time.Minute,
	})
}

type impl struct{}

// Setup will check if rootfs is removed or not and reboots the dut if required.
func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	const (
		devMode           = "--allow-ra-in-dev-mode "
		enableHevc        = "--enable-clear-hevc-for-testing"
		configFile        = "/etc/chrome_dev.conf"
		oemCryptoPath     = "/var/lib/oemcrypto"
		oemPublicCertFile = "oem_public_cert.bin"
		wrappedRSAKeyFile = "wrapped_rsa_key.bin"
		wrappedKeyboxFile = "wrapped_wv_keybox"
	)

	d := s.DUT()
	// Attempt to connect to those DUTs that aren't already connected.
	if !d.Connected(ctx) {
		s.Log("Attempting to connect to DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return d.Connect(ctx)
		}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
			s.Fatal("Failed to connect to DUT: ", err)
		}
		s.Log("Connected to DUT")
	}

	// Connect to local gRPC services, and keep connection alive until after
	// TearDown is called by using the fixture context.
	rpcClient, err := rpc.Dial(s.FixtContext(), s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the local gRPC service on the DUT: ", err)
	}

	rootfsIsWritable, err := sysutil.IsRootfsWritable(ctx, rpcClient)
	if err != nil {
		s.Fatal("Failed to check if rootfs is writable: ", err)
	}
	if !rootfsIsWritable {
		s.Log("Making rootfs writable")
		// Since MakeRootfsWritable will reboot the DUT, closing the RPC
		// and starting the RPC before and after calling MakeRootfsWritable.
		if err := rpcClient.Close(ctx); err != nil {
			s.Fatal("Failed to close rpc: ", err)
		}
		// Rootfs must be writable in order to disable the upstart job.
		if err := sysutil.MakeRootfsWritable(ctx, d, s.RPCHint()); err != nil {
			s.Fatal("Failed to make rootfs writable: ", err)
		}
		rpcClient, err = rpc.Dial(s.FixtContext(), s.DUT(), s.RPCHint())
		if err != nil {
			s.Fatal("Failed to redial rpc: ", err)
		}
	} else {
		s.Log("WARNING: The rootfs is writable")
	}

	out, err := d.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("cat %s", configFile)).Output()
	if err != nil {
		s.Fatal("Failed to run cat config file command: ", err)
	}
	if !(strings.Contains(string(out), devMode) && strings.Contains(string(out), enableHevc)) {
		for _, text := range []string{devMode, enableHevc} {
			if err := d.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("echo '%s' >> %s", text, configFile)).Run(); err != nil {
				s.Fatalf("Failed to write %s to %s: %v", text, configFile, err)
			}
		}
	}

	output, err := d.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("ls %s", oemCryptoPath)).Output()
	if err != nil {
		s.Fatal("Failed to run oemcrypto command: ", err)
	}

	var oemCryptoContents = []string{oemPublicCertFile, wrappedRSAKeyFile, wrappedKeyboxFile}
	for _, val := range oemCryptoContents {
		if !strings.Contains(string(output), val) {
			s.Errorf("Failed to find %s at %s", val, oemCryptoPath)
		}
	}

	if err := rpcClient.Close(ctx); err != nil {
		s.Error("Failed to close gRPC connection to DUT: ", err)
	}
	return nil
}

func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	// No-op
}

func (i *impl) Reset(ctx context.Context) error {
	return nil
}

func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}
