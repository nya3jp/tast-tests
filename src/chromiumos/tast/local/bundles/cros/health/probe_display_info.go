// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeDisplayInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for display info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		BugComponent: "b:982097",
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeDisplayInfo(ctx context.Context, s *testing.State) {
	// When testing, cros_healthd restarts ui. Display needs some time for
	// the initialization. If cros_healthd reads the data before
	// initialization and modetest reads after the initialization, their
	// data can't match. Currently it only happens to bob and scarlet.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if encoderID, err := modetestConnectorInfo(ctx, connectorEncoder); err != nil {
			return err
		} else if encoderID == "0" {
			return errors.New("there is no encoder id for the connector")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Log("there is no encoder id after 10 seconds")
	}

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
	ResolutionHorizontal   *jsontypes.Uint32 `json:"resolution_horizontal"`
	ResolutionVertical     *jsontypes.Uint32 `json:"resolution_vertical"`
	RefreshRate            *float64          `json:"refresh_rate"`
	Manufacturer           string            `json:"manufacturer"`
	ModelID                *uint16           `json:"model_id"`
	SerialNumber           *jsontypes.Uint32 `json:"serial_number"`
	ManufactureWeek        *uint8            `json:"manufacture_week"`
	ManufactureYear        *uint16           `json:"manufacture_year"`
	EdidVersion            string            `json:"edid_version"`
	InputType              string            `json:"input_type"`
	DisplayName            string            `json:"display_name"`
}

type externalDisplayInfo struct {
	DisplayWidth         *jsontypes.Uint32 `json:"display_width"`
	DisplayHeight        *jsontypes.Uint32 `json:"display_height"`
	ResolutionHorizontal *jsontypes.Uint32 `json:"resolution_horizontal"`
	ResolutionVertical   *jsontypes.Uint32 `json:"resolution_vertical"`
	RefreshRate          *float64          `json:"refresh_rate"`
	Manufacturer         string            `json:"manufacturer"`
	ModelID              *uint16           `json:"model_id"`
	SerialNumber         *jsontypes.Uint32 `json:"serial_number"`
	ManufactureWeek      *uint8            `json:"manufacture_week"`
	ManufactureYear      *uint16           `json:"manufacture_year"`
	EdidVersion          string            `json:"edid_version"`
	InputType            string            `json:"input_type"`
	DisplayName          string            `json:"display_name"`
}

type modetestConnectorColumn int

const (
	connectorID       modetestConnectorColumn = 1
	connectorEncoder                          = 2
	connectorStatus                           = 3
	connectorName                             = 4
	connectorSize                             = 5
	connectorModes                            = 6
	connectorEncoders                         = 7
)

type modetestEncoderColumn int

const (
	encoderID             modetestEncoderColumn = 1
	encoderCrtc                                 = 2
	encoderType                                 = 3
	encoderPossibleCrtcs                        = 4
	encoderPossibleClones                       = 5
)

type modetestModeInfoColumn int

const (
	modeInfoID         modetestModeInfoColumn = 1
	modeInfoName                              = 2
	modeInfoVrefresh                          = 3
	modeInfoHdisplay                          = 4
	modeInfoHsyncStart                        = 5
	modeInfoHsyncEnd                          = 6
	modeInfoHtotal                            = 7
	modeInfoVdisplay                          = 8
	modeInfoVSyncStart                        = 9
	modeInfoVsyncEnd                          = 10
	modeInfoVtotal                            = 11
	modeInfoClock                             = 12
)

// edpExists returns true when it detects an embedded display(eDP) on the DUT.
// It returns false when there is no eDP and returns error when it fails to run the command.
func edpExists(ctx context.Context) (bool, error) {
	b, err := testexec.CommandContext(ctx, "modetest", "-c").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}

	modetestOutput := strings.Trim(string(b), "\n")
	// When there is a keyword like "eDP" or "DSI" in the output of modetest, it means the DUT has an eDP.
	// An example for the output from the modetest:
	//     id      encoder status          name            size (mm)       modes   encoders
	//     71      70      connected       eDP-1           290x190         1       70
	eDPKeywords := []string{"eDP", "DSI"}
	for _, keyword := range eDPKeywords {
		if strings.Contains(modetestOutput, keyword) {
			return true, nil
		}
	}

	return false, nil
}

func privacyScreenSupported(ctx context.Context) (bool, error) {
	b, err := testexec.CommandContext(ctx, "modetest", "-c").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	swStateExist := strings.Contains(string(b), "privacy-screen sw-state")
	hwStateExist := strings.Contains(string(b), "privacy-screen hw-state")
	// Both sw-state and hw-state should exist to indicate the feature is supported.
	if swStateExist && hwStateExist {
		return true, nil
	} else if swStateExist || hwStateExist {
		return false, nil
	}

	// Fall back to legacy interface.
	return strings.Contains(string(b), "privacy-screen"), nil
}

func privacyScreenEnabled(ctx context.Context) (bool, error) {
	// Only hw-state indicates the real state of privacy screen info.
	cmd := "modetest -c | sed -n -e '/eDP/,/connected/ p' | grep -A 3 'privacy-screen hw-state' | grep 'value' | awk -e '{ print $2 }'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	hwStateValue := strings.TrimRight(string(b), "\n")
	if hwStateValue != "" {
		return hwStateValue == "1", nil
	}

	// hw-state is empty, we need to fall back to legacy interface.
	cmd = "modetest -c | sed -n -e '/eDP/,/connected/ p' | grep -A 3 'privacy-screen' | grep 'value' | awk -e '{ print $2 }'"
	b, err = testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to run modetest command")
	}

	return strings.TrimRight(string(b), "\n") == "1", nil
}

func verifyPrivacyScreenInfo(ctx context.Context, edp *embeddedDisplayInfo) error {
	privacyScreenSupported, err := privacyScreenSupported(ctx)
	if err != nil {
		return err
	}
	if privacyScreenSupported != edp.PrivacyScreenSupported {
		return errors.Errorf("failed. PrivacyScreenSupported doesn't match: got %v; want %v", edp.PrivacyScreenSupported, privacyScreenSupported)
	}

	if !privacyScreenSupported && edp.PrivacyScreenEnabled {
		return errors.New("failed. Privacy screen is not supported, but privacy_screen_enabled is true")
	}

	privacyScreenEnabled, err := privacyScreenEnabled(ctx)
	if err != nil {
		return err
	}
	if privacyScreenEnabled != edp.PrivacyScreenEnabled {
		return errors.Errorf("failed. PrivacyScreenEnabled doesn't match: got %v; want %v", edp.PrivacyScreenEnabled, privacyScreenEnabled)
	}

	return nil
}

func compareUintPointer[T uint8 | uint16 | uint32](got *T, want T, field string) error {
	if got == nil {
		if want != 0 {
			return errors.Errorf("failed. %s doesn't match: got nil; want %v", field, want)
		}
	} else if want != *got {
		return errors.Errorf("failed. %s doesn't match: got %v; want %v", field, *got, want)
	}

	return nil
}

func modetestConnectorInfo(ctx context.Context, column modetestConnectorColumn) (string, error) {
	// Example output of "modetest -c" (partially):
	// id      encoder status          name            size (mm)       modes   encoders
	// 71      70      connected       eDP-1           290x190         1       70
	//
	// We'll try to get the line that contains "eDP" string first, and get the value at |column| index.
	cmd := "modetest -c | grep -E 'DSI|eDP' | awk -e '{print $" + strconv.Itoa(int(column)) + "}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func modetestEncoderInfo(ctx context.Context, encoderID string, column modetestEncoderColumn) (string, error) {
	// Example output of "modetest -e" (partially):
	// id      crtc    type    possible crtcs  possible clones
	// 70      41      TMDS    0x00000007      0x00000001
	//
	// We'll try to get the line that starts with |encoderID| first, and get the value for crtc ID at column 2.
	cmd := "modetest -e | grep ^" + encoderID + " | awk -e '{print $" + strconv.Itoa(int(column)) + "}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func modetestCrtcInfo(ctx context.Context, crtcID string, column modetestModeInfoColumn) (string, error) {
	// Example output of "modetest -p" (partially):
	// id      fb      pos     size
	// 41      97      (0,0)   (1920x1280)
	//   #0 1920x1280 60.00 1920 1944 1992 2080 1280 1286 1303 1320 164740 flags: nhsync, nvsync; type: preferred, driver
	//
	// We'll try to get the line that starts with |crtcID| first, get the following line as details info, and get the value at |column| index.
	cmd := "modetest -p | grep ^" + crtcID + " -A 1 | sed '1d' | awk -e '{print $" + strconv.Itoa(int(column)) + "}'"
	b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(b), "\n"), nil
}

func modetestModeInfo(ctx context.Context, column modetestModeInfoColumn) (string, error) {
	// Example output of mode info:
	// #0 1920x1280 60.00 1920 1944 1992 2080 1280 1286 1303 1320 164740 flags: nhsync, nvsync; type: preferred, driver
	//
	// We should find the mode info in the following ways:
	// 1. Find the mode info in crtc first, it means the current used mode info.
	// 2. Fall back to the "preferred" mode info in connector info.
	if encoderID, err := modetestConnectorInfo(ctx, connectorEncoder); err != nil {
		return "", err
	} else if encoderID == "0" {
		// It means that we can't find the crtc info. So fall back to method 2.
		cmd := "modetest -c | grep -E 'DSI|eDP' -A 10 | grep preferred | awk -e '{print $" + strconv.Itoa(int(column)) + "}'"
		b, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
		if err != nil {
			return "", err
		}

		return strings.TrimRight(string(b), "\n"), nil
	} else if crtcID, err := modetestEncoderInfo(ctx, encoderID, encoderCrtc); err != nil {
		return "", err
	} else if info, err := modetestCrtcInfo(ctx, crtcID, column); err != nil {
		return "", err
	} else {
		return info, nil
	}
}

func modetestConnectorEdidInfo(ctx context.Context) (string, error) {
	// Example EDID output of "modetest -c" (partially):
	// id      encoder status          name            size (mm)       modes   encoders
	// 32      0       connected       eDP-1           310x170         1       31
	// modes:
	//       index name refresh (Hz) hdisp hss hse htot vdisp vss vse vtot
	// #0 1920x1080 60.05 1920 1936 1952 2104 1080 1083 1097 1116 141000 flags: nhsync, nvsync; type: preferred, driver
	// props:
	//       1 EDID:
	//               flags: immutable blob
	//               blobs:
	//
	//               value:
	//                       00ffffffffffff0006af3d4000000000
	//                       001b0104951f1178029b859256599029
	//                       20505400000001010101010101010101
	//                       010101010101143780b8703824401010
	//                       3e0035ad100000180000000000000000
	//                       00000000000000000000000000fe0041
	//                       554f0a202020202020202020000000fe
	//                       004231343048414e30342e30200a00eb

	b, err := testexec.CommandContext(ctx, "modetest", "-c").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run modetest command")
	}
	tks := strings.Split(string(b), "\n")
	findEdp := false
	for idx, tk := range tks {
		if strings.Contains(tk, "eDP") || strings.Contains(tk, "DSI") {
			findEdp = true
		}
		if findEdp && strings.Contains(tk, "1 EDID:") && idx+13 <= len(tks) {
			cmd := testexec.CommandContext(ctx, "edid-decode", "-s")
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return "", errors.Wrap(err, "failed to open stdin pipe")
			}

			edid := strings.Join(tks[idx+5:idx+13], "")
			io.WriteString(stdin, edid)
			stdin.Close()
			if b, err := cmd.Output(); err == nil {
				return string(b), nil
			}
			return "", errors.Wrapf(err, "failed to get edid-decode output, edid: %s, modetest controller: %s", edid, string(b))
		}
	}
	return "", errors.Errorf("there is no embedded display edid info, modetest controller: %s", string(b))
}

func verifyEmbeddedDisplaySize(ctx context.Context, edp *embeddedDisplayInfo) error {
	if edpExists, err := edpExists(ctx); err != nil {
		return err
	} else if !edpExists {
		if edp.DisplayWidth != nil {
			return errors.New("there is no embedded display, but cros_healthd report DisplayWidth field")
		}
		if edp.DisplayHeight != nil {
			return errors.New("there is no embedded display, but cros_healthd report DisplayHeight field")
		}
		return nil
	}

	sizeRaw, err := modetestConnectorInfo(ctx, connectorSize)
	if err != nil {
		return err
	}

	size := strings.Split(sizeRaw, "x")
	if width, err := strconv.ParseUint(size[0], 10, 32); err != nil {
		return err
	} else if err := compareUintPointer((*uint32)(edp.DisplayWidth), uint32(width), "DisplayWidth"); err != nil {
		return err
	}

	if height, err := strconv.ParseUint(size[1], 10, 32); err != nil {
		return err
	} else if err := compareUintPointer((*uint32)(edp.DisplayHeight), uint32(height), "DisplayHeight"); err != nil {
		return err
	}

	return nil
}

func verifyEmbeddedDisplayResolution(ctx context.Context, edp *embeddedDisplayInfo) error {
	if edpExists, err := edpExists(ctx); err != nil {
		return err
	} else if !edpExists {
		if edp.ResolutionHorizontal != nil {
			return errors.New("there is no embedded display, but cros_healthd report ResolutionHorizontal field")
		}
		if edp.ResolutionVertical != nil {
			return errors.New("there is no embedded display, but cros_healthd report ResolutionVertical field")
		}
		return nil
	}

	if horizontalRaw, err := modetestModeInfo(ctx, modeInfoHdisplay); err != nil {
		return err
	} else if verticalRaw, err := modetestModeInfo(ctx, modeInfoVdisplay); err != nil {
		return err
	} else if horizontalRaw == "" && verticalRaw == "" {
		// It means that we can't get the info in use, or default preferred info.
		// Then we need to check if cros_healthd reports nothing.
		if edp.ResolutionHorizontal != nil || edp.ResolutionVertical != nil {
			return errors.New("there is no resolution info, but cros_healthd report it")
		}
		return nil
	} else if horizontal, err := strconv.ParseUint(horizontalRaw, 10, 32); err != nil {
		return err
	} else if err := compareUintPointer((*uint32)(edp.ResolutionHorizontal), uint32(horizontal), "ResolutionHorizontal"); err != nil {
		return err
	} else if vertical, err := strconv.ParseUint(verticalRaw, 10, 32); err != nil {
		return err
	} else if err := compareUintPointer((*uint32)(edp.ResolutionVertical), uint32(vertical), "ResolutionVertical"); err != nil {
		return err
	}

	return nil
}

func verifyEmbeddedDisplayRefreshRate(ctx context.Context, edp *embeddedDisplayInfo) error {
	if edpExists, err := edpExists(ctx); err != nil {
		return err
	} else if !edpExists {
		if edp.RefreshRate != nil {
			return errors.New("there is no embedded display, but cros_healthd report RefreshRate field")
		}
		return nil
	}

	var wantRefreshRate float64
	if htotalRaw, err := modetestModeInfo(ctx, modeInfoHtotal); err != nil {
		return err
	} else if vtotalRaw, err := modetestModeInfo(ctx, modeInfoVtotal); err != nil {
		return err
	} else if clockRaw, err := modetestModeInfo(ctx, modeInfoClock); err != nil {
		return err
	} else if htotalRaw == "" && vtotalRaw == "" && clockRaw == "" {
		// It means that we can't get the info in use, or default preferred info.
		// Then we need to check if cros_healthd reports nothing.
		if edp.RefreshRate != nil {
			return errors.New("there is no refresh rate info, but cros_healthd report it")
		}
		return nil
	} else if htotal, err := strconv.ParseUint(htotalRaw, 10, 32); err != nil {
		return err
	} else if vtotal, err := strconv.ParseUint(vtotalRaw, 10, 32); err != nil {
		return err
	} else if clock, err := strconv.ParseUint(clockRaw, 10, 32); err != nil {
		return err
	} else {
		wantRefreshRate = float64(clock) * 1000.0 / float64(htotal*vtotal)
	}

	if math.Abs(wantRefreshRate-*edp.RefreshRate) > 0.01 {
		return errors.Errorf("failed. RefreshRate doesn't match: got %v; want %v", *edp.RefreshRate, wantRefreshRate)
	}

	return nil
}

var manufacturerRegexp = regexp.MustCompile(`Manufacturer: (.*)`)
var modelIDRegexp = regexp.MustCompile(`Model: (.*)`)
var serialNumberRegexp = regexp.MustCompile(`Serial Number: (.*)`)

func verifyEmbeddedDisplayIdentifier(ctx context.Context, edp *embeddedDisplayInfo, edidInfo string) error {
	manufacturer := ""
	if manufacturerMatch := manufacturerRegexp.FindStringSubmatch(edidInfo); manufacturerMatch != nil {
		manufacturer = manufacturerMatch[1]
	}
	if manufacturer != edp.Manufacturer {
		return errors.Errorf("failed. Manufacturer doesn't match: got %v; want %v", edp.Manufacturer, manufacturer)
	}

	modelIDRaw := ""
	if modelIDMatch := modelIDRegexp.FindStringSubmatch(edidInfo); modelIDMatch != nil {
		modelIDRaw = modelIDMatch[1]
	}
	if modelIDRaw == "" && edp.ModelID != nil {
		return errors.New("there is no ModelID info, but cros_healthd report it")
	} else if modelIDRaw != "" {
		if modelID, err := strconv.ParseUint(modelIDRaw, 10, 16); err != nil {
			return err
		} else if err := compareUintPointer(edp.ModelID, uint16(modelID), "ModelID"); err != nil {
			return err
		}
	}

	serialNumberRaw := ""
	if serialNumberMatch := serialNumberRegexp.FindStringSubmatch(edidInfo); serialNumberMatch != nil {
		serialNumberRaw = serialNumberMatch[1]
	}
	if serialNumberRaw == "" && edp.SerialNumber != nil {
		return errors.New("there is no SerialNumber info, but cros_healthd report it")
	} else if serialNumberRaw != "" {
		if serialNumber, err := strconv.ParseUint(serialNumberRaw, 10, 32); err != nil {
			return err
		} else if err := compareUintPointer((*uint32)(edp.SerialNumber), uint32(serialNumber), "SerialNumber"); err != nil {
			return err
		}
	}

	return nil
}

var manufactureYearRegexp = regexp.MustCompile(`Made in:.*([0-9]{4})`)
var manufactureWeekRegexp = regexp.MustCompile(`Made in: week (.*) of [0-9]{4}`)

func verifyEmbeddedDisplayManufactureDate(ctx context.Context, edp *embeddedDisplayInfo, edidInfo string) error {
	manufactureYearRaw := ""
	if manufactureYearMatch := manufactureYearRegexp.FindStringSubmatch(edidInfo); manufactureYearMatch != nil {
		manufactureYearRaw = manufactureYearMatch[1]
	}
	if manufactureYearRaw == "" && edp.ManufactureYear != nil {
		return errors.New("there is no ManufactureYear info, but cros_healthd report it")
	} else if manufactureYearRaw != "" {
		if manufactureYear, err := strconv.ParseUint(manufactureYearRaw, 10, 16); err != nil {
			return err
		} else if err := compareUintPointer(edp.ManufactureYear, uint16(manufactureYear), "ManufactureYear"); err != nil {
			return err
		}
	}

	manufactureWeekRaw := ""
	if manufactureWeekMatch := manufactureWeekRegexp.FindStringSubmatch(edidInfo); manufactureWeekMatch != nil {
		manufactureWeekRaw = manufactureWeekMatch[1]
	}
	if manufactureWeekRaw == "" && edp.ManufactureWeek != nil {
		return errors.New("there is no ManufactureWeek info, but cros_healthd report it")
	} else if manufactureWeekRaw != "" {
		if manufactureWeek, err := strconv.ParseUint(manufactureWeekRaw, 10, 8); err != nil {
			return err
		} else if err := compareUintPointer(edp.ManufactureWeek, uint8(manufactureWeek), "ManufactureWeek"); err != nil {
			return err
		}
	}

	return nil
}

var inputTypeRegexp = regexp.MustCompile(`Basic Display Parameters & Features:\n {4}(.*) display`)
var displayNameRegexp = regexp.MustCompile(`Display Product Name: '(.*)'`)

func verifyEmbeddedDisplayProperty(ctx context.Context, edp *embeddedDisplayInfo, edidInfo string) error {
	inputType := ""
	if inputTypeMatch := inputTypeRegexp.FindStringSubmatch(edidInfo); inputTypeMatch != nil {
		inputType = inputTypeMatch[1]
	}
	if inputType != edp.InputType {
		return errors.Errorf("failed. InputType doesn't match: got %v; want %v", edp.InputType, inputType)
	}

	displayName := ""
	if displayNameMatch := displayNameRegexp.FindStringSubmatch(edidInfo); displayNameMatch != nil {
		displayName = displayNameMatch[1]
	}
	if displayName != edp.DisplayName {
		return errors.Errorf("failed. DisplayName doesn't match: got %v; want %v", edp.DisplayName, displayName)
	}

	return nil
}

var edidVersionRegexp = regexp.MustCompile(`EDID Structure Version & Revision: (.*)`)

func verifyEmbeddedDisplayEdid(ctx context.Context, edp *embeddedDisplayInfo) error {
	if edpExists, err := edpExists(ctx); err != nil {
		return err
	} else if !edpExists {
		if edp.ModelID != nil {
			return errors.New("there is no embedded display, but cros_healthd report ModelID field")
		}
		if edp.SerialNumber != nil {
			return errors.New("there is no embedded display, but cros_healthd report SerialNumber field")
		}
		if edp.ManufactureWeek != nil {
			return errors.New("there is no embedded display, but cros_healthd report ManufactureWeek field")
		}
		if edp.ManufactureYear != nil {
			return errors.New("there is no embedded display, but cros_healthd report ManufactureYear field")
		}
		return nil
	}

	edidInfo, err := modetestConnectorEdidInfo(ctx)
	if err != nil {
		if edp.Manufacturer == "" && edp.ModelID == nil && edp.SerialNumber == nil &&
			edp.ManufactureWeek == nil && edp.ManufactureYear == nil &&
			edp.EdidVersion == "" && edp.InputType == "" && edp.DisplayName == "" {
			return nil
		}
		return errors.Wrap(err, "failed to get edid info")
	}

	if err := verifyEmbeddedDisplayIdentifier(ctx, edp, edidInfo); err != nil {
		return errors.Wrap(err, "failed to verify embedded display identifier")
	}
	if err := verifyEmbeddedDisplayManufactureDate(ctx, edp, edidInfo); err != nil {
		return errors.Wrap(err, "failed to verify embedded display manufacture date")
	}
	if err := verifyEmbeddedDisplayProperty(ctx, edp, edidInfo); err != nil {
		return errors.Wrap(err, "failed to verify embedded display property")
	}

	edidVersion := ""
	if edidVersionMatch := edidVersionRegexp.FindStringSubmatch(edidInfo); edidVersionMatch != nil {
		edidVersion = edidVersionMatch[1]
	}
	if edidVersion != edp.EdidVersion {
		return errors.Errorf("failed. EdidVersion doesn't match: got %v; want %v", edp.EdidVersion, edidVersion)
	}

	return nil
}

func verifyEmbeddedDisplayInfo(ctx context.Context, edp *embeddedDisplayInfo) error {
	if err := verifyPrivacyScreenInfo(ctx, edp); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplaySize(ctx, edp); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplayResolution(ctx, edp); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplayRefreshRate(ctx, edp); err != nil {
		return err
	}
	if err := verifyEmbeddedDisplayEdid(ctx, edp); err != nil {
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
