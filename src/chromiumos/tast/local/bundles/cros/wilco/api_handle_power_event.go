// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIHandlePowerEvent,
		Desc: "Tests that the Wilco DTC VM receives power events using the DPSL",
		Contacts: []string{
			"lamzin@google.com", // Test author and wilco_dtc_supportd maintainer
			"pmoy@chromium.org", // wilco_dtc_supportd author
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIHandlePowerEvent(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "powerd"); err != nil {
		s.Fatal("Unable to stop powerd: ", err)
	}
	defer func() {
		if err := upstart.StartJob(ctx, "powerd"); err != nil {
			s.Error("Unable to start powerd: ", err)
		}
	}()

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop()

	waitForPowerEvent := func(expectedEvent dtcpb.HandlePowerNotificationRequest_PowerEvent) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		for {
			s.Log("Waiting for power event: ", expectedEvent)
			msg := dtcpb.HandlePowerNotificationRequest{}
			if err := rec.WaitForMessage(ctx, &msg); err != nil {
				s.Fatal("Unable to receive power event: ", err)
			}
			if msg.PowerEvent != expectedEvent {
				s.Logf("Received power event %v, but waiting for %v. Continuing", msg.PowerEvent, expectedEvent)
				continue
			}
			s.Log("Received power event")
			break
		}
	}

	{
		// Do not wait for the first power event since WilcoDTCSupportd cashes
		// the last external power AC event it sent to the WilcoDTC. That's why
		// there is no guarantee which value is in the cache.
		externalPower := pmpb.PowerSupplyProperties_AC
		power.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		})

		externalPower = pmpb.PowerSupplyProperties_DISCONNECTED
		power.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_AC_REMOVE)

		externalPower = pmpb.PowerSupplyProperties_USB
		power.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_AC_INSERT)
	}

	{
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-1)
		power.EmitSuspendImminent(ctx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		power.EmitSuspendDone(ctx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}

	{
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-2)
		power.EmitDarkSuspendImminent(ctx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		power.EmitSuspendDone(ctx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}
}
