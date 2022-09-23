// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package runtimeprobe contains local tast tests that verifies runtime_probe.
package runtimeprobe

import (
	"context"
	"encoding/json"
	"os"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type verifySandboxTestParams struct {
	probeConfig probeConfig
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySandbox,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks runtime_probe sandbox",
		Contacts: []string{
			"chungsheng@google.com",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"racc"},
		Params: []testing.Param{{
			Val: verifySandboxTestParams{
				probeConfig: probeConfig{[]probeStatement{
					probeStatement{"edid"},
					probeStatement{"generic_battery"},
					probeStatement{"generic_network"},
					probeStatement{"generic_storage"},
					probeStatement{"input_device"},
					probeStatement{"tcpc"},
					probeStatement{"usb_camera"},
				}}},
		}, {
			// These probe function can only be run on amd64.
			Name: "amd64",
			Val: verifySandboxTestParams{
				probeConfig: probeConfig{[]probeStatement{
					probeStatement{"memory"},
				}}},
			ExtraSoftwareDeps: []string{"amd64"},
		}, {
			// These probe functions are unstable and need to be fixed on each device
			// after collects info from informational test results.
			Name: "unstable",
			Val: verifySandboxTestParams{
				probeConfig: probeConfig{[]probeStatement{
					probeStatement{"gpu"},
				}}},
		}},
	})
}

// VerifySandbox verifies runtime_probe can work with sandbox.
func VerifySandbox(ctx context.Context, s *testing.State) {
	testParam := s.Param().(verifySandboxTestParams)
	pc := testParam.probeConfig

	res, err := getRuntimeProbeResult(ctx, pc)
	if err != nil {
		s.Fatal("Failed to run runtime_probe: ", err)
	}
	resFactory, err := getFactoryRuntimeProbeResult(ctx, pc)
	if err != nil {
		s.Fatal("Failed to run factory_runtime_probe: ", err)
	}
	if diff := cmp.Diff(string(res), string(resFactory)); diff != "" {
		s.Fatalf("Result from runtime_probe and factory_runtime_probe mismatch (-want +got): %s", diff)
	}
	if !json.Valid(res) {
		s.Fatalf("Result from both runtime_probe are not valid json: %s", res)
	}
}

type probeConfig struct {
	probeStatements []probeStatement
}

type probeStatement struct {
	probeFunctionName string
}

func (pc probeConfig) MarshalJSON() ([]byte, error) {
	r := make(map[string]map[string]probeStatement)
	for _, ps := range pc.probeStatements {
		comp := make(map[string]probeStatement)
		comp[ps.probeFunctionName+"_name"] = ps
		r[ps.probeFunctionName+"_category"] = comp
	}
	return json.Marshal(r)
}

func (ps probeStatement) MarshalJSON() ([]byte, error) {
	pf := make(map[string]map[string]string)
	pf[ps.probeFunctionName] = make(map[string]string)
	r := make(map[string]map[string]map[string]string)
	r["eval"] = pf
	return json.Marshal(r)
}

func getRuntimeProbeResult(ctx context.Context, pc probeConfig) ([]byte, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, errors.Wrap(err, "cannot create tmp file for runtime probe")
	}
	defer os.Remove(f.Name())

	pcj, err := json.Marshal(pc)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal probe config")
	}

	if _, err := f.Write(pcj); err != nil {
		return nil, errors.Wrap(err, "cannot write probe config")
	}
	if err := f.Close(); err != nil {
		return nil, errors.Wrap(err, "cannot close probe config")
	}

	cmd := testexec.CommandContext(ctx, "runtime_probe", "--config_file_path="+f.Name(), "--to_stdout")
	return cmd.Output(testexec.DumpLogOnError)
}

func getFactoryRuntimeProbeResult(ctx context.Context, pc probeConfig) ([]byte, error) {
	pcj, err := json.Marshal(pc)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal probe config")
	}

	cmd := testexec.CommandContext(ctx, "factory_runtime_probe", string(pcj))
	return cmd.Output(testexec.DumpLogOnError)
}
