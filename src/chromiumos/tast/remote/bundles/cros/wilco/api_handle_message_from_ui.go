// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIHandleMessageFromUi,
		Desc: "Test sending a message from a Chromium extension to the Wilco DTC VM",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		Data:         []string{"wilco_extension_manifest.json"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
		Timeout:      10 * time.Minute,
	})
}

func APIHandleMessageFromUi(ctx context.Context, s *testing.State) { // NOLINT
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := wilco.NewWilcoServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users
	pb.DeviceAffiliationIds = []string{"default"}
	pb.UserAffiliationIds = []string{"default"}

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	manifest, err := ioutil.ReadFile(s.DataPath("wilco_extension_manifest.json"))
	if err != nil {
		s.Fatal("Failed to read extension manifest: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		Extensions: []*ps.Extension{&ps.Extension{
			Id: "emelalhagcpibaiiiijjlkmhhbekaidg",
			Files: []*ps.ExtensionFile{
				&ps.ExtensionFile{
					Name:     "manifest.json",
					Contents: manifest,
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

	if _, err := wc.StartDPSLListener(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create listener: ", err)
	}
	defer wc.StopDPSLListener(ctx, &empty.Empty{})

	if _, err := pc.EvalStatementInExtension(ctx, &ps.EvalInExtensionRequest{
		Id:         "emelalhagcpibaiiiijjlkmhhbekaidg",
		Expression: "chrome.runtime.sendNativeMessage('com.google.wilco_dtc', {'test': 1})",
	}); err != nil {
		s.Fatal("Failed to send native message: ", err)
	}

	s.Log("Waiting for HandleMessageFromUi")
	eventCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := wc.WaitForHandleMessageFromUi(eventCtx, &empty.Empty{}); err != nil {
		s.Error("Did not recieve HandleMessageFromUi event: ", err)
	}
}
