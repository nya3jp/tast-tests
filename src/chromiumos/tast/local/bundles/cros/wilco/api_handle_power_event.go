// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIHandlePowerEvent,
		Desc: "Test sending invalid gRPC requests with enums out of range from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIHandlePowerEvent(ctx context.Context, s *testing.State) {
	conn, err := dbus.SystemBus()
	if err != nil {
		s.Fatal("Cannot connect to system bus: ", err)
	}

	owned := dbusutil.ServiceOwned(ctx, conn, "org.chromium.PowerManager")
	s.Log("Owned: ", owned)

	if err := upstart.StopJob(ctx, "powerd"); err != nil {
		s.Fatal("unable to stop the %s service: %v", "powerd", err)
	}
	defer upstart.RestartJob(ctx, "powerd")

	reply, err := conn.RequestName("org.chromium.PowerManager", dbus.NameFlagReplaceExisting)
	if err != nil {
		s.Fatal("Cannot request ownership: ", reply, err)
	}
	s.Log("RequestName reply: ", reply)

	owned = dbusutil.ServiceOwned(ctx, conn, "org.chromium.PowerManager")
	s.Log("Owned: ", owned)

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop()

	externalPower := pmpb.PowerSupplyProperties_DISCONNECTED
	// externalPower := pmpb.PowerSupplyProperties_AC
	powerMessage := pmpb.PowerSupplyProperties{
		ExternalPower: &externalPower,
	}
	bytes, err := proto.Marshal(&powerMessage)
	if err != nil {
		s.Fatal("Cannot marshal proto to byte array: ", err)
	}

	// arr := []byte{0x28, 0x00, 0x30, 0x00, 0x39, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x59, 0x40, 0x60, 0x00, 0x70, 0x00, 0x78, 0x00, 0x81, 0x01, 0xd2, 0x35, 0x93, 0x6f, 0xb6, 0xb9, 0x81, 0xbf, 0x8a, 0x01, 0x02, 0x41, 0x43, 0x92, 0x01, 0x17, 0x0a, 0x02, 0x41, 0x43, 0x18, 0x01, 0x22, 0x00, 0x2a, 0x00, 0x31, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x38, 0x00, 0x40, 0x01, 0x98, 0x01, 0x00, 0xa1, 0x01, 0x8f, 0xc2, 0xf5, 0x28, 0x5c, 0x4f, 0x21, 0x40, 0xaa, 0x01, 0x03, 0x42, 0x59, 0x44, 0xb0, 0x01, 0x03, 0xba, 0x01, 0x04, 0x32, 0x34, 0x31, 0x36, 0xc1, 0x01, 0x14, 0xae, 0x47, 0xe1, 0x7a, 0x94, 0x1f, 0x40, 0xc9, 0x01, 0xc0, 0xca, 0xa1, 0x45, 0xb6, 0xf3, 0x1d, 0x40, 0xd1, 0x01, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x1e, 0x40, 0xd9, 0x01, 0xc0, 0xca, 0xa1, 0x45, 0xb6, 0xf3, 0x1d, 0x40, 0xe2, 0x01, 0x0c, 0x44, 0x45, 0x4c, 0x4c, 0x20, 0x4e, 0x32, 0x4b, 0x36, 0x32, 0x39, 0x35}
	err = conn.Emit(dbus.ObjectPath("/org/chromium/PowerManager"), "org.chromium.PowerManager.PowerSupplyPoll", bytes)
	if err != nil {
		s.Fatal("Cannot emit event: ", err)
	}

	s.Log("Waiting for power Notification")
	msg := dtcpb.HandlePowerNotificationRequest{}
	if err := rec.WaitForMessage(ctx, &msg); err != nil {
		s.Fatal("Unable to receive power response: ", err)
	}
	s.Log("Received Power Notification: ", msg)
}
