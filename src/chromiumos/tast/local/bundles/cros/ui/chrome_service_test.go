// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	gotesting "testing"

	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/ui"
)

func TestToOptions(t *gotesting.T) {
	for _, tc := range []struct {
		name         string
		request      *pb.NewRequest
		expectedOpts []chrome.Option
	}{
		{
			name: "fill fields",
			request: &pb.NewRequest{
				EnableFeatures:        []string{"a"},
				DisableFeatures:       []string{"b"},
				ExtraArgs:             []string{"--my-flag"},
				LacrosEnableFeatures:  []string{"lacrosa"},
				LacrosDisableFeatures: []string{"lacrosb"},
				LacrosExtraArgs:       []string{"--my-lacros-flag"}},
			expectedOpts: []chrome.Option{
				chrome.EnableFeatures("a"),
				chrome.DisableFeatures("b"),
				chrome.ExtraArgs("--my-flag"),
				chrome.LacrosEnableFeatures("lacrosa"),
				chrome.LacrosDisableFeatures("lacrosb"),
				chrome.LacrosExtraArgs("--my-lacros-flag"),
			},
		}, {
			name: "real login",
			request: &pb.NewRequest{
				LoginMode: pb.LoginMode_LOGIN_MODE_GAIA_LOGIN,
				Credentials: &pb.NewRequest_Credentials{
					Username: "user",
					Password: "pass",
					GaiaId:   "123",
				},
			},
			expectedOpts: []chrome.Option{
				chrome.GAIALogin(chrome.Creds{User: "user", Pass: "pass", GAIAID: "123"}),
			},
		}} {
		t.Run(tc.name, func(t *gotesting.T) {
			opts, err := toOptions(tc.request)
			if err != nil {
				t.Error("Unable to convert to options: ", err)
			}
			diff, err := chrome.OptionsDiff(opts, tc.expectedOpts)
			if err != nil {
				t.Error("Invalid options: ", err)
			}
			if diff != "" {
				t.Errorf("Options mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
