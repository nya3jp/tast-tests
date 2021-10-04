// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeDisplayInfo,
		Desc: "Check that we can probe cros_healthd for display info",
		Contacts: []string{
			"cros-tdm@google.com",
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeDisplayInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryDisplay}
	var display displayInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &display); err != nil {
		s.Fatal("Failed to get display telemetry info: ", err)
	}

	if err := verifyDisplayData(ctx, display); err != nil {
		s.Fatal("Failed to validate display data, err: ", err)
	}
}

type displayInfo struct {
	EDP embeddedDisplayInfo `json:"edp"`
}

type embeddedDisplayInfo struct {
	PrivacyScreenEnabled   bool `json:"privacy_screen_enabled"`
	PrivacyScreenSupported bool `json:"privacy_screen_supported"`
}

func isPrivacyScreenSupported(ctx context.Context) (bool, error) {
	cmd := "modetest -c | grep 'privacy-screen'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	if string(b) != "" {
		return true, nil
	}

	return false, nil
}

func isPrivacyScreenEnabled(ctx context.Context) (bool, error) {
	cmd := "modetest -c | sed -n -e '/eDP/,/connected/ p' | grep -A 3 'privacy-screen' | grep 'value' | awk -e '{ print $2 }'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	return strings.TrimRight(string(b), "\n") == "1", nil
}

func verifyEmbeddedDisplayInfo(ctx context.Context, EDP embeddedDisplayInfo) error {
	privacyScreenSupported, err := isPrivacyScreenSupported(ctx)
	if err != nil {
		return err
	}
	if privacyScreenSupported != EDP.PrivacyScreenSupported {
		return errors.Errorf("failed. PrivacyScreenSupported doesn't match: got %v; want %v", EDP.PrivacyScreenSupported, privacyScreenSupported)
	}

	if !privacyScreenSupported && EDP.PrivacyScreenEnabled {
		return errors.New("Failed. Privacy screen is not supported, but privacy_screen_enabled is true")
	}

	privacyScreenEnabled, err := isPrivacyScreenEnabled(ctx)
	if err != nil {
		return err
	}
	if privacyScreenEnabled != EDP.PrivacyScreenEnabled {
		return errors.Errorf("failed. PrivacyScreenEnabled doesn't match: got %v; want %v", EDP.PrivacyScreenEnabled, privacyScreenEnabled)
	}

	return nil
}

func verifyDisplayData(ctx context.Context, display displayInfo) error {
	if err := verifyEmbeddedDisplayInfo(ctx, display.EDP); err != nil {
		return err
	}

	return nil
}
