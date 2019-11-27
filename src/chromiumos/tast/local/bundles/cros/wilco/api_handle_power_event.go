// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/power"
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
	emitter, err := power.NewPowerManagerEmitter(ctx)
	if err != nil {
		s.Fatal("Unable to create power manager emitter: ", err)
	}
	defer func() {
		if err := emitter.Stop(ctx); err != nil {
			s.Error("Unable to stop emitter: ", err)
		}
	}()

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop()

	waitForPowerEvent := func(expectedEvent dtcpb.HandlePowerNotificationRequest_PowerEvent) {
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
		externalPower := pmpb.PowerSupplyProperties_DISCONNECTED
		emitter.EmitPowerSupplyPoll(&pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		})

		externalPower = pmpb.PowerSupplyProperties_AC
		emitter.EmitPowerSupplyPoll(&pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_AC_INSERT)
	}

	{
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-1)
		emitter.EmitSuspendImminent(&pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		emitter.EmitSuspendDone(&pmpb.SuspendDone{
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}

	{
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-2)
		emitter.EmitDarkSuspendImminent(&pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		emitter.EmitSuspendDone(&pmpb.SuspendDone{
			SuspendId: &suspendID,
		})
		waitForPowerEvent(dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}
}
