// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"encoding/base64"
	"time"

	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/remote/bluetooth"
	pb "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

const testVarFastPairAntispoofingKeyPem = "fast_pair_antispoofing_key_pem"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FastPairInitialPair,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the Fast Pair initial pairing scenario",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BTTestService"},
		Fixture:      "chromeLoggedInAsUserWithFastPairAnd1BTPeer",
		Timeout:      2 * time.Minute,
		Vars:         []string{testVarFastPairAntispoofingKeyPem},
	})
}

// FastPairInitialPair tests the Fast Pair initial pairing scenario.
func FastPairInitialPair(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Parse antispoofing key pem from test var.
	antispoofingKeyPemBase64 := s.RequiredVar(testVarFastPairAntispoofingKeyPem)
	antispoofingKeyPem, err := base64.StdEncoding.DecodeString(antispoofingKeyPemBase64)
	if err != nil {
		s.Fatalf("Failed to decode %q from base64 string: %v", testVarFastPairAntispoofingKeyPem, err)
	}

	// Configure btpeer as a fast pair device.
	testing.ContextLog(ctx, "Configuring btpeer as a fast pair device with an antispoofing key pem set")
	fastPairDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0], &bluetooth.EmulatedBTPeerDeviceConfig{
		DeviceType: cbt.DeviceTypeLEFastPair,
	})
	if err != nil {
		s.Fatal("Failed to configure btpeer as a fast pair device: ", err)
	}
	if err := fastPairDevice.RPCFastPair().SetAntispoofingKeyPem(ctx, antispoofingKeyPem); err != nil {
		s.Fatal("Failed to set antispoofing key pem on fast pair btpeer: ", err)
	}

	// TODO automate click through UI fast pair notification to pair device instead of manual wait
	testing.ContextLog(ctx, "Waiting for manual pairing click")
	if err := testing.Sleep(ctx, 90*time.Second); err != nil {
		s.Fatal("Failed to wait for manual pairing click")
	}

	testing.ContextLog(ctx, "Confirming target device is paired")
	resp, err := fv.BTS.DeviceStatus(ctx, &pb.DeviceStatusRequest{
		Device: fastPairDevice.BTSDevice(),
	})
	if err != nil {
		s.Fatal("Failed to check if target device is paired: ", err)
	}
	if !resp.IsPaired {
		s.Fatal("Fast pair device not paired as expected")
	}
}
