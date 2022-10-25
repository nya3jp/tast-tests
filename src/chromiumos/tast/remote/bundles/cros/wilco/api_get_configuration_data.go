// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIGetConfigurationData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test sending GetConfigurationData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
			"tast.cros.wilco.WilcoService",
		},
		Timeout: 7 * time.Minute,
	})
}

func APIGetConfigurationData(ctx context.Context, s *testing.State) {
	const configData = `{"test": 1}`
	const newConfigData = `{"test": 2}`

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.StartExternalDataServer(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start a URLPolicyServer: ", err)
	}
	defer pc.StopExternalDataServer(ctx, &empty.Empty{})

	createPolicyBlob := func(configData string) []byte {
		res, err := pc.ServePolicyData(ctx, &ps.ServePolicyDataRequest{
			Contents: []byte(configData),
		})
		if err != nil {
			s.Fatal("Failed to serve policy: ", err)
		}

		pb := policy.NewBlob()
		pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
		pb.AddPolicy(&policy.DeviceWilcoDtcConfiguration{
			Val: &policy.DeviceWilcoDtcConfigurationValue{
				Url:  res.Url,
				Hash: string(res.Hash),
			},
		})
		// wilco_dtc and wilco_dtc_supportd only run for affiliated users
		pb.DeviceAffiliationIds = []string{"default"}
		pb.UserAffiliationIds = []string{"default"}

		pJSON, err := json.Marshal(pb)
		if err != nil {
			s.Fatal("Failed to serialize policies: ", err)
		}

		return pJSON
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: createPolicyBlob(configData),
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	wc := wilco.NewWilcoServiceClient(cl.Conn)

	preStatus, err := wc.GetStatus(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Could not get running status: ", err)
	} else if preStatus.WilcoDtcSupportdPid == 0 {
		s.Fatal("Wilco DTC Supportd not running")
	}

	if data, err := wc.GetConfigurationData(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to perform GetConfigurationData: ", err)
	} else if data.JsonConfigurationData != configData {
		s.Errorf("Unexpected policy value: got %s, want %s", data.JsonConfigurationData, configData)
	}

	postStatus, err := wc.GetStatus(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Could not get running status: ", err)
	} else if postStatus.WilcoDtcSupportdPid == 0 {
		s.Fatal("Wilco DTC Supportd not running")
	}

	if !proto.Equal(preStatus, postStatus) {
		s.Errorf("wilco_dtc PID changed after request: before %v, after %v", preStatus, postStatus)
	}

	if _, err = wc.RestartVM(ctx, &wilco.RestartVMRequest{
		StartProcesses: false,
		TestDbusConfig: false,
	}); err != nil {
		s.Fatal("Failed to restart the VM without processes: ", err)
	}

	if _, err := wc.StartDPSLListener(ctx, &wilco.StartDPSLListenerRequest{}); err != nil {
		s.Fatal("Failed to create listener: ", err)
	}
	defer wc.StopDPSLListener(ctx, &empty.Empty{})

	s.Log("Updating policies")
	if _, err := pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
		PolicyJson: createPolicyBlob(newConfigData),
	}); err != nil {
		s.Fatal("Failed to update policy: ", err)
	}

	s.Log("Waiting for HandleConfigurationDataChanged")
	eventCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := wc.WaitForHandleConfigurationDataChanged(eventCtx, &empty.Empty{}); err != nil {
		s.Error("Did not recieve HandleConfigurationDataChanged event: ", err)
	}

	if data, err := wc.GetConfigurationData(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to perform GetConfigurationData: ", err)
	} else if data.JsonConfigurationData != newConfigData {
		s.Errorf("Unexpected policy value: got %s, want %s", data.JsonConfigurationData, newConfigData)
	}
}
