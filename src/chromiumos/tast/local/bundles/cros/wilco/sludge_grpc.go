// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	wvm "chromiumos/tast/local/bundles/cros/wilco/vm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	dtc "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SludgeGRPC,
		Desc: "Starts an instance of the Wilco DTC VM and exercises the gRPC boundary to a daemon on the host, wilco_dtc_supportd",
		Contacts: []string{
			"tbegin@chromium.org", // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.org",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func SludgeGRPC(ctx context.Context, s *testing.State) {
	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Expect the services to start within 5 seconds.
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := wvm.StartSludge(startCtx, false); err != nil {
		s.Fatal("Unable to Start Sludge VM: ", err)
	}
	defer wvm.StopSludge(cleanupCtx)

	if err := wvm.StartWilcoSupportDaemon(startCtx); err != nil {
		s.Fatal("Unable to start wilco_dtc_supportd: ", err)
	}
	defer wvm.StopWilcoSupportDaemon(cleanupCtx)

	m := jsonpb.Marshaler{}

	osMsg := dtc.GetOsVersionRequest{}
	str, err := m.MarshalToString(&osMsg)
	if err != nil {
		s.Error("Unable to convert proto msg to JSON: ", err)
	}

	s.Log("Checking VM -> Host Communication")
	out, err := dpslUtilSend(ctx, "GetOsVersion", str)
	if err != nil {
		s.Error("Unable to get OS version: ", err)
	} else {
		s.Logf("Successfully got OS version: %s", out)
		osRes := dtc.GetOsVersionResponse{}
		parseDPSLUtilMsg(out, &osRes)

		// Error conditions defined by the proto definition.
		if len(osRes.Version) == 0 {
			s.Error("OS Version is blank")
		}
		if osRes.Milestone == 0 {
			s.Error("OS Milestone is 0")
		}
	}

	ecMsg := dtc.GetEcTelemetryRequest{}
	// Get EC firmware label following kernel driver
	// https://chromium.googlesource.com/chromiumos/third_party/kernel/+/d145cca29f845e55e353cbb86fa9391a71f71dbb/drivers/platform/chrome/wilco_ec/sysfs.c?pli=1#48
	ecMsg.Payload = []byte{0x38, 0x00, 0x00}
	str, err = m.MarshalToString(&ecMsg)
	if err != nil {
		s.Error("Unable to Marshal EC Payload to String: ", err)
	}

	s.Log("Getting EC Telemetry")
	out, err = dpslUtilSend(ctx, "GetEcTelemetry", str)
	if err != nil {
		s.Error("Unable to get EC Telemetry: ", err)
	} else {
		s.Logf("Successfully got EC Telemetry: %s", out)
		ecRes := dtc.GetEcTelemetryResponse{}
		parseDPSLUtilMsg(out, &ecRes)

		if ecRes.Status != dtc.GetEcTelemetryResponse_STATUS_OK {
			s.Errorf("Unexpected EC telemetry response status: got %s, want GetEcTelemetryResponse_STATUS_OK", ecRes.Status)
		}
	}
}

// dpslUtilSend is a helper function that creates and runs a
// diagnostics_dpsl_test_requester command over vsh. It accepts the name of
// command and the JSON body to pass to it. It returns the stdout of the
// command. See https://crrev.com/c/1767044 for a description of the
// diagnostics_dpsl_test_requester.
func dpslUtilSend(ctx context.Context, name, body string) ([]byte, error) {
	cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, "diagnostics_dpsl_test_requester",
		"--message_name="+name, fmt.Sprintf("--message_body=%s", body))
	return cmd.Output(testexec.DumpLogOnError)
}

// parseDPSLUtilMsg takes a message received from the
// diagnostics_dpsl_test_requester and converts it into the provided proto
// definition.
func parseDPSLUtilMsg(msg []byte, pb proto.Message) error {
	var jsn map[string]interface{}
	if err := json.Unmarshal(msg, &jsn); err != nil {
		return errors.Wrap(err, "unable to parse byte message to JSON")
	}

	jsnStr, err := json.Marshal(jsn["body"])
	if err != nil {
		return errors.Wrap(err, "unable to parse JSON body")
	}
	if err := jsonpb.UnmarshalString(fmt.Sprintf("%s", jsnStr), pb); err != nil {
		return errors.Wrap(err, "unable to parse JSON to proto")
	}
	return nil
}
