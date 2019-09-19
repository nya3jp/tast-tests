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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	wilco "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SludgeGRPC,
		Desc: "Starts an instance of the Wilco DTC VM and exercises the gRPC boundary to a daemon on the host, wilco_dtc_supportd",
		Contacts: []string{
			"tbegin@chromium.org", // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func SludgeGRPC(ctx context.Context, s *testing.State) {
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := vm.StartSludge(startCtx, false); err != nil {
		s.Error(err, "unable to start sludge VM")
	}
	defer vm.StopSludge(ctx)

	if err := vm.StartWilcoSupportDaemon(startCtx); err != nil {
		s.Error(err, "unable to start wilco_dtc_supportd")
	}
	defer vm.StopWilcoSupportDaemon(ctx)

	m := jsonpb.Marshaler{}

	osMsg := wilco.GetOsVersionRequest{}
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
	}

	ecMsg := wilco.GetEcTelemetryRequest{}
	ecMsg.Payload = []byte{0x38, 0x00, 0x00} // Get EC firmware label
	str, err = m.MarshalToString(&ecMsg)

	s.Log("Getting EC Telemetry")
	out, err = dpslUtilSend(ctx, "GetEcTelemetry", str)
	if err != nil {
		s.Error("Unable to get EC Telemetry: ", err)
	} else {
		s.Logf("Successfully got EC Telemetry: %s", out)
	}

	var jsn map[string]interface{}
	if err := json.Unmarshal(out, &jsn); err != nil {
		s.Error("Unable to parse EC telemetry message to JSON: ", err)
	}

	ecRes := wilco.GetEcTelemetryResponse{}
	jsnStr, err := json.Marshal(jsn["body"])
	if err != nil {
		s.Error("Unable to parse JSON body from EC Telemetry")
	}
	if err := jsonpb.UnmarshalString(fmt.Sprintf("%s", jsnStr), &ecRes); err != nil {
		s.Error("Unable to parse EC telemetry JSON to proto: ", err)
	}

	if ecRes.Status != wilco.GetEcTelemetryResponse_STATUS_OK {
		s.Error("EC telemetry response status not OK")
	}
}

func dpslUtilSend(ctx context.Context, cmd string, body string) ([]byte, error) {
	out, err := vm.SendVSHCommand(ctx, vm.WilcoVMCID, "diagnostics_dpsl_test_requester",
		"--message_name="+cmd, fmt.Sprintf("--message_body=%s", body))
	if err != nil {
		return out, errors.Wrap(err, "DPSL Util send failed")
	}

	return out, nil
}
