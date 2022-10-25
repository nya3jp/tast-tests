// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wilco/wilcoextension"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIHandleMessageFromUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test sending a message from a Chromium extension to the Wilco DTC VM",
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
		Timeout: 10 * time.Minute,
	})
}

// APIHandleMessageFromUI tests Wilco DTC HandleMessageFromUi gRPC API.
// TODO(b/189457904): remove once wilco.APIHandleMessageFromUIEnrolled will be stable enough.
func APIHandleMessageFromUI(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := wilco.NewWilcoServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pb := policy.NewBlob()
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users
	pb.DeviceAffiliationIds = []string{"default"}
	pb.UserAffiliationIds = []string{"default"}

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		Extensions: []*ps.Extension{{
			Id: wilcoextension.ID,
			Files: []*ps.ExtensionFile{
				{
					Name:     "manifest.json",
					Contents: []byte(wilcoextension.Manifest),
				},
				{
					Name:     "background.js",
					Contents: []byte{},
				},
			},
		}},
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if _, err = wc.RestartVM(ctx, &wilco.RestartVMRequest{
		StartProcesses: false,
		TestDbusConfig: false,
	}); err != nil {
		s.Fatal("Failed to restart the VM without processes: ", err)
	}

	type testMsg struct {
		Test int
	}

	vmResponse := testMsg{
		Test: 5,
	}

	marshaled, err := json.Marshal(vmResponse)
	if err != nil {
		s.Fatal("Failed to marshal message: ", err)
	}

	if _, err := wc.StartDPSLListener(ctx, &wilco.StartDPSLListenerRequest{
		HandleMessageFromUiResponse: string(marshaled),
	}); err != nil {
		s.Fatal("Failed to create listener: ", err)
	}
	defer wc.StopDPSLListener(ctx, &empty.Empty{})

	nm, err := wilcoextension.NewBuiltInMessaging(ctx, pc)
	if err != nil {
		s.Fatal("Failed to start built-in messaging: ", err)
	}

	uiRequest := testMsg{
		Test: 5,
	}

	s.Log("Sending message from extension")
	sendMessageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var uiResponse testMsg
	if err := nm.SendMessageAndGetReply(sendMessageCtx, &uiRequest, &uiResponse); err != nil {
		s.Fatal("Failed to send message using extension: ", err)
	}

	if uiResponse != vmResponse {
		s.Errorf("Unexpected response received: got %v; want %v", uiResponse, vmResponse)
	}

	s.Log("Waiting for HandleMessageFromUi")
	eventCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	msg, err := wc.WaitForHandleMessageFromUi(eventCtx, &empty.Empty{})
	if err != nil {
		s.Error("Did not recieve HandleMessageFromUi event: ", err)
	}

	var vmRequest testMsg
	if err := json.Unmarshal([]byte(msg.JsonMessage), &vmRequest); err != nil {
		s.Fatalf("Failed to unmarshall %q: %v", msg.JsonMessage, err)
	}

	if uiRequest != vmRequest {
		s.Errorf("Unexpected message received: got %v; want %v", vmRequest, uiRequest)
	}
}
