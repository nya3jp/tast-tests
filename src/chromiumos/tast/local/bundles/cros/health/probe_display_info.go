// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeDisplayInfo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that we can probe cros_healthd for display info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
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

	if err := verifyDisplayData(ctx, &display); err != nil {
		s.Fatal("Failed to validate display data, err: ", err)
	}
}

type displayInfo struct {
	EDP embeddedDisplayInfo    `json:"edp"`
	DP  *[]externalDisplayInfo `json:"dp"`
}

type embeddedDisplayInfo struct {
	PrivacyScreenEnabled   bool              `json:"privacy_screen_enabled"`
	PrivacyScreenSupported bool              `json:"privacy_screen_supported"`
	DisplayWidth           *jsontypes.Uint32 `json:"display_width"`
	DisplayHeight          *jsontypes.Uint32 `json:"display_height"`
	ResolutionWidth        *jsontypes.Uint32 `json:"resolution_width"`
	ResolutionHeight       *jsontypes.Uint32 `json:"resolution_height"`
	RefreshRate            *jsontypes.Uint32 `json:"refresh_rate"`
}

type externalDisplayInfo struct {
	DisplayWidth     *jsontypes.Uint32 `json:"display_width"`
	DisplayHeight    *jsontypes.Uint32 `json:"display_height"`
	ResolutionWidth  *jsontypes.Uint32 `json:"resolution_width"`
	ResolutionHeight *jsontypes.Uint32 `json:"resolution_height"`
	RefreshRate      *jsontypes.Uint32 `json:"refresh_rate"`
}

func isPrivacyScreenSupported(ctx context.Context) (bool, error) {
	b, err := testexec.CommandContext(ctx, "modetest", "-c").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	return strings.Contains(string(b), "privacy-screen"), nil
}

func isPrivacyScreenEnabled(ctx context.Context) (bool, error) {
	cmd := "modetest -c | sed -n -e '/eDP/,/connected/ p' | grep -A 3 'privacy-screen' | grep 'value' | awk -e '{ print $2 }'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	return strings.TrimRight(string(b), "\n") == "1", nil
}

func verifyPrivacyScreenInfo(ctx context.Context, EDP *embeddedDisplayInfo) error {
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

func compareUint32Pointer(got *jsontypes.Uint32, want uint32, field string) error {
	if got == nil {
		if want != 0 {
			return errors.Errorf("failed. %s doesn't match: got nil; want %v", field, want)
		}
	} else if want != uint32(*got) {
		return errors.Errorf("failed. %s doesn't match: got %v; want %v", field, *got, want)
	}

	return nil
}

func getModetestConnectorInfo(ctx context.Context, column int) (string, error) {
	// Example output of "modetest -c" (partially):
	// id      encoder status          name            size (mm)       modes   encoders
	// 71      70      connected       eDP-1           290x190         1       70
	//
	// We'll try to get the line that contains "eDP" string first, and get the value at |column| index.
	cmd := "modetest -c | grep eDP | awk -e '{print $" + strconv.Itoa(column) + "}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func verifyEmbeddedDisplaySize(ctx context.Context, EDP *embeddedDisplayInfo) error {
	sizeRaw, err := getModetestConnectorInfo(ctx, 5)
	if err != nil {
		return err
	}

	size := strings.Split(sizeRaw, "x")
	width, _ := strconv.ParseUint(size[0], 10, 32)
	height, _ := strconv.ParseUint(size[1], 10, 32)
	if err := compareUint32Pointer(EDP.DisplayWidth, uint32(width), "DisplayWidth"); err != nil {
		return err
	}
	if err := compareUint32Pointer(EDP.DisplayHeight, uint32(height), "DisplayHeight"); err != nil {
		return err
	}

	return nil
}

func getEmbeddedDisplayCrtcID(ctx context.Context) (string, error) {
	encoderID, err := getModetestConnectorInfo(ctx, 2)
	if err != nil {
		return "", err
	}

	// Example output of "modetest -e" (partially):
	// id      crtc    type    possible crtcs  possible clones
	// 70      41      TMDS    0x00000007      0x00000001
	//
	// We'll try to get the line that starts with |encoderID| first, and get the value for crtc ID at column 2.
	cmd := "modetest -e | grep ^" + encoderID + " | awk -e '{print $2}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func getModetestCrtcInfo(ctx context.Context, crtcID string, column int) (string, error) {
	// Example output of "modetest -p" (partially):
	// id      fb      pos     size
	// 41      97      (0,0)   (1920x1280)
	//   #0 1920x1280 60.00 1920 1944 1992 2080 1280 1286 1303 1320 164740 flags: nhsync, nvsync; type: preferred, driver
	//
	// We'll try to get the line that starts with |crtcID| first, get the following line as details info, and get the value at |column| index.
	cmd := "modetest -p | grep ^" + crtcID + " -A 1 | sed '1d' | awk -e '{print $" + strconv.Itoa(column) + "}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func verifyEmbeddedDisplayResolutionSize(ctx context.Context, EDP *embeddedDisplayInfo) error {
	crtcID, err := getEmbeddedDisplayCrtcID(ctx)
	if err != nil {
		return err
	}

	sizeRaw, err := getModetestCrtcInfo(ctx, crtcID, 2)
	if err != nil {
		return err
	}

	size := strings.Split(sizeRaw, "x")
	width, _ := strconv.ParseUint(size[0], 10, 32)
	height, _ := strconv.ParseUint(size[1], 10, 32)
	if err := compareUint32Pointer(EDP.ResolutionWidth, uint32(width), "ResolutionWidth"); err != nil {
		return err
	}
	if err := compareUint32Pointer(EDP.ResolutionHeight, uint32(height), "ResolutionHeight"); err != nil {
		return err
	}

	return nil
}

func verifyEmbeddedDisplayInfo(ctx context.Context, EDP *embeddedDisplayInfo) error {
	if err := verifyPrivacyScreenInfo(ctx, EDP); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplaySize(ctx, EDP); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplayResolutionSize(ctx, EDP); err != nil {
		return err
	}

	return nil
}

func verifyDisplayData(ctx context.Context, display *displayInfo) error {
	if err := verifyEmbeddedDisplayInfo(ctx, &display.EDP); err != nil {
		return err
	}

	return nil
}
