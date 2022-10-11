// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package featured

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type featureState struct {
	FeatureName                  string
	EnabledCallbackEnabledResult bool
	ParamsCallbackFeatureName    string
	ParamsCallbackEnabledResult  bool
	ParamsCallbackParamsResult   map[string]string
}

type featureLibraryTestParams struct {
	Name                    string
	ChromeParam             string
	ExpectedDefaultEnabled  featureState
	ExpectedDefaultDisabled featureState
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FeatureLibraryLateBoot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify features are enabled/disabled as expected and parameters are unchanged",
		Contacts: []string{
			"kendraketsui@google.com",
			"mutexlox@google.com",
			"cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "experiment_enabled_without_params",
			Val: featureLibraryTestParams{
				ChromeParam: "--enable-features=CrOSLateBootTestDefaultEnabled,CrOSLateBootTestDefaultDisabled",
				ExpectedDefaultEnabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultEnabled",
					EnabledCallbackEnabledResult: true,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultEnabled",
					ParamsCallbackEnabledResult:  true,
					ParamsCallbackParamsResult:   map[string]string{},
				},
				ExpectedDefaultDisabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultDisabled",
					EnabledCallbackEnabledResult: true,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultDisabled",
					ParamsCallbackEnabledResult:  true,
					ParamsCallbackParamsResult:   map[string]string{},
				},
			},
		}, {
			Name: "experiment_disabled_without_params",
			Val: featureLibraryTestParams{
				ChromeParam: "--disable-features=CrOSLateBootTestDefaultEnabled,CrOSLateBootTestDefaultDisabled",
				ExpectedDefaultEnabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultEnabled",
					EnabledCallbackEnabledResult: false,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultEnabled",
					ParamsCallbackEnabledResult:  false,
					ParamsCallbackParamsResult:   map[string]string{},
				},
				ExpectedDefaultDisabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultDisabled",
					EnabledCallbackEnabledResult: false,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultDisabled",
					ParamsCallbackEnabledResult:  false,
					ParamsCallbackParamsResult:   map[string]string{},
				},
			},
		}, {
			Name: "experiment_enabled_with_params",
			Val: featureLibraryTestParams{
				ChromeParam: "--enable-features=CrOSLateBootTestDefaultEnabled:k1/v1/k2/v2,CrOSLateBootTestDefaultDisabled:k3/v3/k4/v4",
				ExpectedDefaultEnabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultEnabled",
					EnabledCallbackEnabledResult: true,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultEnabled",
					ParamsCallbackEnabledResult:  true,
					ParamsCallbackParamsResult: map[string]string{
						"k1": "v1",
						"k2": "v2"},
				},
				ExpectedDefaultDisabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultDisabled",
					EnabledCallbackEnabledResult: true,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultDisabled",
					ParamsCallbackEnabledResult:  true,
					ParamsCallbackParamsResult: map[string]string{
						"k3": "v3",
						"k4": "v4"},
				},
			},
		}, {
			Name: "experiment_default",
			Val: featureLibraryTestParams{
				ChromeParam: "",
				ExpectedDefaultEnabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultEnabled",
					EnabledCallbackEnabledResult: true,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultEnabled",
					ParamsCallbackEnabledResult:  true,
					ParamsCallbackParamsResult:   map[string]string{},
				},
				ExpectedDefaultDisabled: featureState{
					FeatureName:                  "CrOSLateBootTestDefaultDisabled",
					EnabledCallbackEnabledResult: false,
					ParamsCallbackFeatureName:    "CrOSLateBootTestDefaultDisabled",
					ParamsCallbackEnabledResult:  false,
					ParamsCallbackParamsResult:   map[string]string{},
				},
			},
		}},
	})
}

// checkExpectedResults checks if the queried feature state matches the expected state
func checkExpectedResults(out []byte, expectedEnabledTest, expectedDisabledTest featureState) error {
	splitOut := bytes.Split(out, []byte("\n"))

	var enabledTestResult featureState
	if err := json.Unmarshal(splitOut[0], &enabledTestResult); err != nil {
		return errors.Wrap(err, "could not unmarshal json result")
	}
	if diff := cmp.Diff(enabledTestResult, expectedEnabledTest); diff != "" {
		return errors.Errorf("Results mismatch for default-enabled feature (-got +want): %s", diff)
	}

	var disabledTestResult featureState
	if err := json.Unmarshal(splitOut[1], &disabledTestResult); err != nil {
		return errors.Wrap(err, "could not unmarshal json result")
	}
	if diff := cmp.Diff(disabledTestResult, expectedDisabledTest); diff != "" {
		return errors.Errorf("Results mismatch for default-disabled feature (-got +want): %s", diff)
	}

	return nil
}

func FeatureLibraryLateBoot(ctx context.Context, s *testing.State) {
	params := s.Param().(featureLibraryTestParams)

	cr, err := chrome.New(ctx, chrome.ExtraArgs(params.ChromeParam), chrome.NoLogin())

	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(ctx)

	cmd := testexec.CommandContext(ctx, "/usr/local/libexec/tast/helpers/local/cros/featured.FeatureLibraryLateBoot.check")

	stdout, stderr, err := cmd.SeparatedOutput()
	const fileName = "check_stderr"
	if err2 := ioutil.WriteFile(filepath.Join(s.OutDir(), fileName), stderr, 0644); err2 != nil {
		s.Error("Failed to write stderr: ", err2)
	}

	if err != nil {
		s.Fatal("One of the feature names was not found after querying feature_library: ", err)
	}

	if err := checkExpectedResults(stdout, params.ExpectedDefaultEnabled, params.ExpectedDefaultDisabled); err != nil {
		s.Fatal("Failed to get the correct output: ", err)
	}
}
