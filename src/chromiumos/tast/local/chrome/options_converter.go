// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"chromiumos/tast/local/chrome/internal/config"
	pb "chromiumos/tast/services/cros/ui"
)

// ToOptions converts a chrome_service.NewRequest into a set of chrome options.
func ToOptions(req *pb.NewRequest) ([]Option, error) {
	var options []Option

	if req.KeepState {
		options = append(options, KeepState())
	}

	if req.TryReuseSession {
		options = append(options, TryReuseSession())
	}

	switch req.GetLoginMode() {
	case pb.LoginMode_LOGIN_MODE_NO_LOGIN:
		options = append(options, NoLogin())
	case pb.LoginMode_LOGIN_MODE_FAKE_LOGIN:
		options = append(options, FakeLogin(toCreds(req.Credentials)))
	case pb.LoginMode_LOGIN_MODE_GAIA_LOGIN:
		options = append(options, GAIALogin(toCreds(req.Credentials)))
	case pb.LoginMode_LOGIN_MODE_GUEST_LOGIN:
		options = append(options, GuestLogin())
	default:
		options = append(options, FakeLogin(Creds{User: config.DefaultUser, Pass: config.DefaultPass}))
	}

	if len(req.ExtraArgs) > 0 {
		options = append(options, ExtraArgs(req.ExtraArgs...))
	}

	if len(req.EnableFeatures) > 0 {
		options = append(options, EnableFeatures(req.EnableFeatures...))
	}

	if len(req.DisableFeatures) > 0 {
		options = append(options, DisableFeatures(req.DisableFeatures...))
	}

	if len(req.LacrosExtraArgs) > 0 {
		options = append(options, LacrosExtraArgs(req.LacrosExtraArgs...))
	}

	if len(req.LacrosEnableFeatures) > 0 {
		options = append(options, LacrosEnableFeatures(req.LacrosEnableFeatures...))
	}

	if len(req.LacrosDisableFeatures) > 0 {
		options = append(options, LacrosDisableFeatures(req.LacrosDisableFeatures...))
	}

	return options, nil
}

func toCreds(c *pb.NewRequest_Credentials) Creds {
	return Creds{
		User:       c.Username,
		Pass:       c.Password,
		GAIAID:     c.GaiaId,
		Contact:    c.Contact,
		ParentUser: c.ParentUsername,
		ParentPass: c.ParentPassword,
	}
}
