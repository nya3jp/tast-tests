// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/jsonpb"

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

	config := wvm.DefaultSludgeConfig()
	config.StartProcesses = false
	if err := wvm.StartSludge(startCtx, config); err != nil {
		s.Fatal("Unable to Start Sludge VM: ", err)
	}
	defer wvm.StopSludge(cleanupCtx)

	if err := wvm.StartWilcoSupportDaemon(startCtx); err != nil {
		s.Fatal("Unable to start wilco_dtc_supportd: ", err)
	}
	defer wvm.StopWilcoSupportDaemon(cleanupCtx)

	if err := testOsVersion(ctx, s); err != nil {
		s.Error("testOSVersion failed: ", err)
	}

	if err := testECTelemetry(ctx, s); err != nil {
		s.Error("testECTelemetry failed: ", err)
	}
}

func testOsVersion(ctx context.Context, s *testing.State) error {
	osMsg := dtc.GetOsVersionRequest{}
	osRes := dtc.GetOsVersionResponse{}

	s.Log("Getting OS Version")
	if err := dpslUtilSend(ctx, "GetOsVersion", &osMsg, &osRes); err != nil {
		return errors.Wrap(err, "unable to get OS version")
	}

	// Error conditions defined by the proto definition.
	if len(osRes.Version) == 0 {
		return errors.Errorf("OS Version is blank: %s", osRes.String())
	}
	if osRes.Milestone == 0 {
		return errors.Errorf("OS Milestone is 0: %s", osRes.String())
	}

	s.Log("Successfully Received OS Version: ", osRes.String())
	return nil
}

func testECTelemetry(ctx context.Context, s *testing.State) error {
	ecMsg := dtc.GetEcTelemetryRequest{}
	// Get EC firmware label following kernel driver
	// https://chromium.googlesource.com/chromiumos/third_party/kernel/+/d145cca29f845e55e353cbb86fa9391a71f71dbb/drivers/platform/chrome/wilco_ec/sysfs.c?pli=1#48
	ecMsg.Payload = []byte{0x38, 0x00, 0x00}
	ecRes := dtc.GetEcTelemetryResponse{}

	s.Log("Getting EC Telemetry")
	if err := dpslUtilSend(ctx, "GetEcTelemetry", &ecMsg, &ecRes); err != nil {
		return errors.Wrap(err, "unable to get EC Telemetry")
	}

	if ecRes.Status != dtc.GetEcTelemetryResponse_STATUS_OK {
		return errors.Errorf(
			"unexpected EC telemetry response status: got %s, want GetEcTelemetryResponse_STATUS_OK", ecRes.Status)
	}

	s.Log("Successfully Received EC Telemetry: ", ecRes.String())
	return nil
}

// dpslUtilSend is a helper function that creates and runs a
// diagnostics_dpsl_test_requester command over vsh. It accepts the name of
// command, the proto message to send, and a proto message to hold the output.
// See https://crrev.com/c/1767044 for a description of the
// diagnostics_dpsl_test_requester.
func dpslUtilSend(ctx context.Context, msgName string, in descriptor.Message, out descriptor.Message) error {
	m := jsonpb.Marshaler{}
	body, err := m.MarshalToString(in)
	if err != nil {
		_, md := descriptor.ForMessage(in)
		return errors.Wrapf(err, "unable to marshal %s to String", md.GetName())
	}

	cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, "diagnostics_dpsl_test_requester",
		"--message_name="+msgName, fmt.Sprintf("--message_body=%s", body))

	msg, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "unable to run diagnostics_dpsl_test_requester")
	}

	if err := parseDPSLUtilMsg(msg, out); err != nil {
		_, md := descriptor.ForMessage(out)
		return errors.Wrapf(err, "error parsing return msg to %s", md.GetName())
	}

	return nil
}

// parseDPSLUtilMsg takes a message received from the
// diagnostics_dpsl_test_requester and converts it into the provided proto
// definition.
func parseDPSLUtilMsg(msg []byte, pb descriptor.Message) error {
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
