// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func ctsExclusions() []string {
	return []string{
		"TestRandomGraph/SingleOperationTest.RESIZE_BILINEAR_V1_3/*", // b/182264329
		"TestGenerated/*svdf_bias_present*",                          // b/189803299
		// Intel exclusions follow
		"*fully_connected_quant8_signed",                             // b/192301242
		"*fully_connected_quant8_signed_2",                           // b/192301242
		"*fully_connected_quant8_signed_all_inputs_as_internal_2",    // b/192301242
		"*DynamicOutputShapeTest*",                                   // b/192301246
		"*QuantizationCouplingTest*",                                 // b/192301071
		"*DeviceMemory*",                                             // b/192301073
		"TestRandomGraph/SingleOperationTest.FULLY_CONNECTED_V1_3/7", // b/192301247
		"*RandomGraphTest.LargeGraph_TENSOR_FLOAT32_Rank3/35",        // b/192301075
		"*TestRandomGraph/RandomGraphTest.LargeGraph*",               // b/192301250
		"*TestRandomGraph/RandomGraphTest.Small*",                    // b/192301077
	}
}

func vtsExclusions() []string {
	return []string{
		"TestRandomGraph/SingleOperationTest.RESIZE_BILINEAR_V1_3/*", // b/182264329
		"TestGenerated/*svdf_bias_present*",                          // b/189803299
		// Intel exclusions follow
		"*fully_connected_quant8_signed",                           // b/192301242
		"*fully_connected_quant8_signed_2",                         // b/192301242
		"*fully_connected_quant8_signed_all_inputs_as_internal_2",  // b/192301242
		"*DynamicOutputShapeTest*",                                 // b/192301246
		"*QuantizationCouplingTest*",                               // b/192301071
		"*Test/cros_default_quantize_quant8_signed_quant8_signed*", // b/192301078
		"*MemoryDomain*",                                           // b/192301169
		"*Fenced*",                                                 // b/192301255
		"*quantize*",                                               // b/192301256
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:    NNAPIDriver,
		Desc:    "Validates that the HAL driver conforms to the NNAPI specification",
		Timeout: 10 * time.Minute,
		Contacts: []string{
			"jmpollock@google.com",
			"slangley@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		SoftwareDeps: []string{"nnapi"},
		Params: []testing.Param{
			{
				Name:              "cts",
				Val:               []string{"cros_nnapi_cts", "--gtest_filter=-" + strings.Join(ctsExclusions(), ":")},
				ExtraSoftwareDeps: []string{"nnapi_vendor_driver"},
			},
			{
				Name:              "vts_1_3",
				Val:               []string{"cros_nnapi_vts_1_3", "--gtest_filter=-" + strings.Join(vtsExclusions(), ":")},
				ExtraSoftwareDeps: []string{"nnapi_vendor_driver"},
			}},
		/*
			TODO: Enable once b/192302431 is resolved (VTS < 3 currently unsupported by Intel, fix underway)
			{
				Name: "vts_1_0",
				Val: []string{"cros_nnapi_vts_1_0", "--gtest_filter=-" + strings.Join(vtsExclusions(), ":")},
			}, {
				Name: "vts_1_1",
				Val: []string{"cros_nnapi_vts_1_1", "--gtest_filter=-" + strings.Join(vtsExclusions(), ":")},
			}, {
				Name: "vts_1_2",
				Val:               []string{"cros_nnapi_vts_1_2", "--gtest_filter=-" + strings.Join(vtsExclusions(), ":")},
				ExtraSoftwareDeps: []string{"nnapi_vendor_driver"},
			},
		*/
	})
}

func NNAPIDriver(ctx context.Context, s *testing.State) {
	cmdArgs := s.Param().([]string)

	cmd := testexec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = append(os.Environ(), "ANDROID_LOG_TAGS=*:f")

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to run test suite: ", err)
	}
}
