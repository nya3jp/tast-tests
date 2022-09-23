// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularCustomApn,
		Desc: "Verifies that traffic can be sent over the Cellular network when using custom APNs",
		Contacts: []string{
			"andrewlassalle@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Data:    []string{"callbox_no_apns.pbf"},
		Fixture: "cellular",
	})
}

func ShillCellularCustomApn(ctx context.Context, s *testing.State) {
	modbOverrideProto := "callbox_no_apns.pbf"
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}
	operatorID, err := modem.GetOperatorIdentifier(ctx)
	if err != nil || len(operatorID) == 0 {
		// ModemManager often fails to read the operator identifier after a cold boot.
		// Try to get the value from the IMSI before failing the test.
		imsi, err := modem.GetIMSI(ctx)
		if err != nil || len(imsi) < 6 {
			s.Fatal("Could not get operator identifier from sim: ", err)
		}
		testing.ContextLog(ctx, "operatorID= ", operatorID, "  imsi= ", imsi)
		operatorID = imsi[0:6]
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	modem3gpp, err := modem.GetModem3gpp(ctx)
	if err != nil {
		s.Fatal("Could not get modem3gpp object: ", err)
	}
	// Try to start the test with a bad attach APN to ensure that we shill clears the attach APN if needed.
	// Some modems don't allow custom attach APNs, so a failure here should not fail the test.
	if err := modemmanager.SetInitialEpsBearerSettings(ctx, modem3gpp, map[string]interface{}{"apn": "wrong_attach", "ip-type": mmconst.BearerIPFamilyIPv4}); err != nil {
		testing.ContextLog(ctx, "Failed to set initial EPS bearer settings: ", err)
	}

	if _, err = helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}

	deferCleanUp, err := cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer deferCleanUp()

	if errs := helper.ResetShill(ctx); errs != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	if _, err = helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable cellular: ", err)
	}

	knownAPNs, err := cellular.GetKnownAPNsForOperator(operatorID)
	if err != nil {
		s.Fatal("Cannot find known APNs: ", err)
	}

	optionalAPNExist := false
	optionalAPNSucceeded := false
	for _, knownAPN := range knownAPNs {
		ipType, okIPType := knownAPN.APNInfo[shillconst.DevicePropertyCellularAPNInfoApnIPType]
		attach, okAttach := knownAPN.APNInfo[shillconst.DevicePropertyCellularAPNInfoApnAttach]
		auth, okAuth := knownAPN.APNInfo[shillconst.DevicePropertyCellularAPNInfoApnAuthentication]
		if okAttach && attach == shillconst.DevicePropertyCellularAPNInfoApnAttachTrue {
			if (okIPType && (ipType == shillconst.DevicePropertyCellularAPNInfoApnIPTypeIPv4v6 || ipType == shillconst.DevicePropertyCellularAPNInfoApnIPTypeIPv6)) ||
				(okAuth && (auth == shillconst.DevicePropertyCellularAPNInfoApnAuthenticationPAP)) {
				// Skip known ipv4v6 and ipv6 APNs, since Cellular.APN doesn't support the ip_type field.
				// Skip known PAP APNs, since Cellular.APN doesn't support the authentication field.
				continue
			}
		}

		if okIPType {
			// Remove ip_type since the UI never sends that value.
			delete(knownAPN.APNInfo, shillconst.DevicePropertyCellularAPNInfoApnIPType)
		}
		if okAuth {
			// Remove authentication since the UI never sends that value.
			delete(knownAPN.APNInfo, shillconst.DevicePropertyCellularAPNInfoApnAuthentication)
		}
		if knownAPN.Optional {
			optionalAPNExist = true
		}
		if err = helper.SetAPN(ctx, knownAPN.APNInfo); err != nil {
			s.Fatal("Unable to set the custom APN: ", err)
		}
		// Because Reattach can be triggered based on the current value and previous |attach| value,
		// it's hard to calculate when the current service will be destroyed and recreated.
		// A 2 second delay is enough to ensure that the service is destroyed when a Reattach is triggered.
		testing.Sleep(ctx, 2*time.Second)
		// Because of Reattach, the service changes when an attach APN is changed.
		service, err := helper.FindServiceForDevice(ctx)
		if err != nil {
			s.Fatal("Unable to find Cellular Service for Device: ", err)
		}

		testing.ContextLog(ctx, "Connecting with ", knownAPN.APNInfo)
		if isConnected, err := service.IsConnected(ctx); err != nil {
			s.Fatal("Unable to get IsConnected for Service: ", err)
		} else if !isConnected {
			if err := helper.ConnectToServiceWithTimeout(ctx, service, 14*time.Second); err != nil {
				if knownAPN.Optional {
					continue
				}
				s.Fatal("Unable to Connect to Service: ", err)
			}
		}

		serviceLastAttachAPN, err := helper.GetCellularLastAttachAPN(ctx)
		if err != nil {
			s.Fatal("Error getting Service properties: ", err)
		}
		serviceLastGoodAPN, err := helper.GetCellularLastGoodAPN(ctx)
		if err != nil {
			s.Fatal("Error getting Service properties: ", err)
		}
		testing.ContextLog(ctx, "serviceLastAttachAPN:", serviceLastAttachAPN)
		testing.ContextLog(ctx, "serviceAPN", serviceLastGoodAPN)

		apn := serviceLastAttachAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
		expectedAPN := knownAPN.APNInfo[shillconst.DevicePropertyCellularAPNInfoApnName]
		isAttach := knownAPN.APNInfo[shillconst.DevicePropertyCellularAPNInfoApnAttach]
		if isAttach == shillconst.DevicePropertyCellularAPNInfoApnAttachTrue && apn != expectedAPN {
			if knownAPN.Optional {
				continue
			}
			s.Fatalf("Last Attach APN doesn't match: got %q, want %q", apn, expectedAPN)
		}

		apn = serviceLastGoodAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
		if apn != expectedAPN {
			if knownAPN.Optional {
				continue
			}
			s.Fatalf("Last good APN doesn't match: got %q, want %q", apn, expectedAPN)
		}
		if knownAPN.Optional {
			optionalAPNSucceeded = true
		}
	}

	if optionalAPNExist && !optionalAPNSucceeded {
		s.Fatal("None of the optional APNs connected")
	}
}
