// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"reflect"
	gotesting "testing"

	"chromiumos/tast/local/chrome/internal/config"
	pb "chromiumos/tast/services/cros/ui"
)

// defaultCreds is the default credentials used for fake logins.
var defaultCreds = Creds{
	User:   config.DefaultUser,
	Pass:   config.DefaultPass,
	GAIAID: "dae9c7c55697ba170d6b494c458649bd469af525520280d0dcfc98d74d13b17e",
}

func TestConvertOptions(t *gotesting.T) {
	for _, tc := range []struct {
		name                  string
		request               *pb.NewRequest
		enableFeatures        []string
		disableFeatures       []string
		extraArgs             []string
		lacrosEnableFeatures  []string
		lacrosDisableFeatures []string
		lacrosExtraArgs       []string
		creds                 Creds
		loginMode             config.LoginMode
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
			enableFeatures:        []string{"a"},
			disableFeatures:       []string{"b"},
			extraArgs:             []string{"--my-flag"},
			lacrosEnableFeatures:  []string{"lacrosa"},
			lacrosDisableFeatures: []string{"lacrosb"},
			lacrosExtraArgs:       []string{"--my-lacros-flag"},
			creds:                 defaultCreds,
			loginMode:             config.FakeLogin,
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
			creds: config.Creds{
				User:   "user",
				Pass:   "pass",
				GAIAID: "123",
			},
			loginMode: config.GAIALogin,
		}} {
		t.Run(tc.name, func(t *gotesting.T) {
			opts, err := ToOptions(tc.request)
			if err != nil {
				t.Error("Unable to convert to options: ", err)
			}
			cfg, err := config.NewConfig(opts)
			if err != nil {
				t.Error("Unable to generate config: ", err)
			}
			if !reflect.DeepEqual(cfg.EnableFeatures(), tc.enableFeatures) {
				t.Errorf("EnableFeatures: got %+v; want %+v", cfg.EnableFeatures(), tc.enableFeatures)
			}
			if !reflect.DeepEqual(cfg.DisableFeatures(), tc.disableFeatures) {
				t.Errorf("Disablefeatures: got %+v; want %+v", cfg.DisableFeatures(), tc.disableFeatures)
			}
			if !reflect.DeepEqual(cfg.ExtraArgs(), tc.extraArgs) {
				t.Errorf("ExtraArgs: got %+v; want %+v", cfg.ExtraArgs(), tc.extraArgs)
			}
			if !reflect.DeepEqual(cfg.LacrosEnableFeatures(), tc.lacrosEnableFeatures) {
				t.Errorf("LacrosEnableFeatures: got %+v; want %+v", cfg.LacrosEnableFeatures(), tc.lacrosEnableFeatures)
			}
			if !reflect.DeepEqual(cfg.LacrosDisableFeatures(), tc.lacrosDisableFeatures) {
				t.Errorf("LacrosDisableFeatures: got %+v; want %+v", cfg.LacrosDisableFeatures(), tc.lacrosDisableFeatures)
			}
			if !reflect.DeepEqual(cfg.LacrosExtraArgs(), tc.lacrosExtraArgs) {
				t.Errorf("LacrosExtraArgs: got %+v; want %+v", cfg.LacrosExtraArgs(), tc.lacrosExtraArgs)
			}
			if !reflect.DeepEqual(cfg.Creds(), tc.creds) {
				t.Errorf("Creds: got %+v; want %+v", cfg.Creds(), tc.creds)
			}
			if cfg.LoginMode() != tc.loginMode {
				t.Errorf("LoginMode: got %d; want %d", cfg.LoginMode(), tc.loginMode)
			}
		})
	}
}
