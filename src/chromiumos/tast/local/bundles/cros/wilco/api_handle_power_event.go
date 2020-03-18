// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
	defer rec.Stop(ctx)

	waitForPowerEvent := func(ctx context.Context, expectedEvent dtcpb.HandlePowerNotificationRequest_PowerEvent) error {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		for {
			s.Log("Waiting for power event: ", expectedEvent)
			msg := dtcpb.HandlePowerNotificationRequest{}
			if err := rec.WaitForMessage(ctx, &msg); err != nil {
				return errors.Wrap(err, "unable to receive power event")
			}
			if msg.PowerEvent != expectedEvent {
				s.Logf("Received power event %v, but waiting for %v. Continuing", msg.PowerEvent, expectedEvent)
				continue
			}
			s.Log("Received power event")
			break
		}

		return nil
	}

	// Shorten the total context by 20 seconds to allow for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// TODO(crbug.com/1062564)
	testing.Poll(shortCtx, func(ctx context.Context) error {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Do not wait for the first power event since WilcoDTCSupportd cashes
		// the last external power AC event it sent to the WilcoDTC. That's why
		// there is no guarantee which value is in the cache.
		externalPower := pmpb.PowerSupplyProperties_AC
		if err := emitter.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll AC: ", err)
		}

		externalPower = pmpb.PowerSupplyProperties_DISCONNECTED
		if err := emitter.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll DISCONNECTED: ", err)
		}
		if err := waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_AC_REMOVE); err != nil {
			return err
		}

		externalPower = pmpb.PowerSupplyProperties_USB
		if err := emitter.EmitPowerSupplyPoll(ctx, &pmpb.PowerSupplyProperties{
			ExternalPower: &externalPower,
		}); err != nil {
			s.Fatal("Failed to emit PowerSupplyPoll USB: ", err)
		}
		return waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_AC_INSERT)
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	testing.Poll(shortCtx, func(ctx context.Context) error {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-1)
		if err := emitter.EmitSuspendImminent(ctx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit SuspendImminent: ", err)
		}
		if err := waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_OS_SUSPEND); err != nil {
			return err
		}

		if err := emitter.EmitSuspendDone(ctx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit SuspendDone: ", err)
		}
		return waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	testing.Poll(shortCtx, func(ctx context.Context) error {
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		reason := pmpb.SuspendImminent_IDLE
		suspendID := int32(-2)
		if err := emitter.EmitDarkSuspendImminent(ctx, &pmpb.SuspendImminent{
			Reason:    &reason,
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit DarkSuspendImminent: ", err)
		}
		if err := waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_OS_SUSPEND); err != nil {
			return err
		}

		if err := emitter.EmitSuspendDone(ctx, &pmpb.SuspendDone{
			SuspendId: &suspendID,
		}); err != nil {
			s.Fatal("Failed to emit DarkSuspendDone: ", err)
		}
		return waitForPowerEvent(ctx, dtcpb.HandlePowerNotificationRequest_OS_RESUME)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}
