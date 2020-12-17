// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/platform/mlbenchmark"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// tfliteDriverPath is the path to the TFlite driver (installed by the ml package).
// This driver isn't installed in the standard lib dir so we specify the full path.
const tfliteDriverPath = "/usr/local/ml_benchmark/ml_service/libml_for_benchmark.so"

type benchmarkParams struct {
	driver     string        // name of the driver (.so file)
	configFile string        // config file for the driver
	scenario   string        // name of the scenario
	tflite     *tfliteParams // if non-nil, then points to the TFlite-specific configuration.
}

type tfliteParams struct {
	archive string // name of the ExtraData archive.
	numRuns int    // number of times to run the model.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmark,
		Desc: "Verifies that the ML Benchmarks work end to end",
		Contacts: []string{
			"jmpollock@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:crosbolt",
			"crosbolt_nightly",
		},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"ml_benchmark"},
		HardwareDeps: hwdep.D(hwdep.ForceDischarge()),
		Params: []testing.Param{
			{
				Name: "soda_no_nnapi",
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1.config",
					scenario:   "soda_no_nnapi",
				},
			},
			{
				Name:              "soda_no_nnapi_goldmont",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("octopus", "volteer")),
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1-goldmont.config",
					scenario:   "soda_no_nnapi_goldmont",
				},
			},
			{
				Name: "soda_no_nnapi_skylake",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("hatch", "volteer"),
					hwdep.SkipOnModel("akemi", "kindred", "kled", "nightfury")),
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1-skylake.config",
					scenario:   "soda_no_nnapi_skylake",
				},
			},
			{
				Name:              "soda_no_nnapi_tigerlake",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer")),
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1-tigerlake.config",
					scenario:   "soda_no_nnapi_tigerlake",
				},
			},
			{
				Name:              "soda_no_nnapi_armv8",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("kukui")),
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1-armv8-a.config",
					scenario:   "soda_no_nnapi_armv8",
				},
			},
			{
				Name: "handwriting_no_nnapi",
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi",
				},
			},
			{
				Name:              "handwriting_no_nnapi_goldmont",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("octopus", "volteer")),
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark-goldmont.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi_goldmont",
				},
			},
			{
				Name: "handwriting_no_nnapi_skylake",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("hatch", "volteer"),
					hwdep.SkipOnModel("akemi", "kindred", "kled", "nightfury")),
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark-skylake.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi_skylake",
				},
			},
			{
				Name:              "handwriting_no_nnapi_tigerlake",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer")),
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark-tigerlake.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi_tigerlake",
				},
			},
			{
				Name:              "handwriting_no_nnapi_armv8",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("kukui")),
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark-armv8-a.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi_armv8",
				},
			},
			{
				Name:      "smartdim_no_nnapi",
				ExtraData: []string{"ml_benchmark_smartdim.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_smartdim.tar.xz",
						numRuns: 1000,
					},
					scenario: "smartdim_no_nnapi",
				},
			},
			{
				Name:      "mobilenet_v2_no_nnapi",
				ExtraData: []string{"ml_benchmark_mobilenet_v2.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_mobilenet_v2.tar.xz",
						numRuns: 1000,
					},
					scenario: "mobilenet_v2_no_nnapi",
				},
			},
			{
				Name:      "mobilenet_v2_quant_no_nnapi",
				ExtraData: []string{"ml_benchmark_mobilenet_v2_quant.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_mobilenet_v2_quant.tar.xz",
						numRuns: 1000,
					},
					scenario: "mobilenet_v2_quant_no_nnapi",
				},
			},
			{
				Name:      "inception_v4_no_nnapi",
				ExtraData: []string{"ml_benchmark_inception_v4.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_inception_v4.tar.xz",
						numRuns: 100,
					},
					scenario: "inception_v4_no_nnapi",
				},
			},
			{
				Name:      "inception_v4_quant_no_nnapi",
				ExtraData: []string{"ml_benchmark_inception_v4_quant.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_inception_v4_quant.tar.xz",
						numRuns: 100,
					},
					scenario: "inception_v4_quant_no_nnapi",
				},
			},
			{
				Name:      "resnet_v2_no_nnapi",
				ExtraData: []string{"ml_benchmark_resnet_v2.tar.xz"},
				Val: benchmarkParams{
					tflite: &tfliteParams{
						archive: "ml_benchmark_resnet_v2.tar.xz",
						numRuns: 100,
					},
					scenario: "resnet_v2_no_nnapi",
				},
			},
		},
	})
}

func MLBenchmark(ctx context.Context, s *testing.State) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("MLBenchmark")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	p, ok := s.Param().(benchmarkParams)
	if !ok {
		s.Fatal("Failed to convert test params to benchmarkParams")
	}

	workspacePath := "/usr/local/ml_benchmark"
	driver := p.driver
	configFile := p.configFile
	scenario := p.scenario

	if p.tflite != nil {
		if len(driver) > 0 {
			s.Fatal("in TFlite configs we expect benchmarkParams.driver to be empty, got ", driver)
		}
		if len(configFile) > 0 {
			s.Fatal("in TFlite configs we expect benchmarkParams.configFile to be empty, got ", configFile)
		}

		archive := s.DataPath(p.tflite.archive)
		tmpDir, err := ioutil.TempDir("", "ml_benchmark")
		if err != nil {
			s.Fatal(err, "failed to create test directory")
		}
		defer os.RemoveAll(tmpDir)

		// --strip-components=1 is to remove the top-level directory that's expected be present in the archive.
		tarCmd := testexec.CommandContext(ctx, "tar", "--strip-components=1", "-xvf", archive, "-C", tmpDir)
		if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal(err, "failed to untar test artifacts")
		}

		configText := fmt.Sprintf(
			`tflite_model_filepath: "%s"
input_output_filepath: "%s"
num_runs: %d`,
			filepath.Join(tmpDir, "model_spec.pb"),
			filepath.Join(tmpDir, "input_output.pb"),
			p.tflite.numRuns)

		configFile = "tflite.config"
		if err := ioutil.WriteFile(filepath.Join(tmpDir, configFile), []byte(configText), 0644); err != nil {
			s.Fatal(err, "failed saving perf data")
		}

		workspacePath = tmpDir
		driver = tfliteDriverPath
	}

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, nil, setup.PowerTestOptions{
		Wifi:    setup.DisableWifiInterfaces,
		Battery: setup.ForceBatteryDischarge,
		// Since we stop the UI disabling the Night Light is redundant.
		NightLight: setup.DoNotDisableNightLight,
	}))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	// Stop UI in order to minimize the number of factors that could influence the results.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.StartJob(ctx, "ui")

	if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait CPU cool down: ", err)
	}

	if err := mlbenchmark.ExecuteScenario(ctx, s.OutDir(), workspacePath, driver, configFile, scenario); err != nil {
		s.Fatalf("Error occurred running the benchmark: %+v", err)
	}
}
