// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/ctxutil"
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
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
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
	defer rec.Stop(ctx)

	waitForPowerEvent := func(ctx context.Context, expectedEvent dtcpb.HandlePowerNotificationRequest_PowerEvent) {
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

	// Shorten the total context by 20 seconds to allow for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Repeat tests to make sure they're not influenced by system events.
	for i := 0; i < 10; i++ {
		// Do not wait for the first power event since WilcoDTCSupportd cashes
		// the last external power AC event it sent to the WilcoDTC. That's why
		// there is no guarantee which value is in the cache.
		externalPower := pmpb.PowerSupplyProperties_AC
		if err := emitter.EmitPowerSupplyPoll(shortCtx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll AC: ", err)
		}

		externalPower = pmpb.PowerSupplyProperties_DISCONNECTED
		if err := emitter.EmitPowerSupplyPoll(shortCtx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll DISCONNECTED: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_AC_REMOVE)

		externalPower = pmpb.PowerSupplyProperties_USB
		if err := emitter.EmitPowerSupplyPoll(shortCtx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll USB: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_AC_INSERT)
	}

	for i := 0; i < 10; i++ {
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-1)
		if err := emitter.EmitSuspendImminent(shortCtx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit SuspendImminent: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		if err := emitter.EmitSuspendDone(shortCtx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit SuspendDone: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}

	for i := 0; i < 10; i++ {
		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-2)
		if err := emitter.EmitDarkSuspendImminent(shortCtx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit DarkSuspendImminent: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_OS_SUSPEND)

		if err := emitter.EmitSuspendDone(shortCtx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit DarkSuspendDone: ", err)
		}
		waitForPowerEvent(shortCtx, dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}
}
