// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// These are the Cr50 Servo controls which can be get/set with a string value.
const (
	GSCCCDLevel    StringControl = "gsc_ccd_level"
	CR50Testlab    StringControl = "cr50_testlab"
	CR50UARTCmd    StringControl = "cr50_uart_cmd"
	CR50UARTRegexp StringControl = "cr50_uart_regexp"
	CR50UARTStream StringControl = "cr50_uart_stream"
)

// These controls accept only "on" and "off" as values.
const (
	CR50UARTCapture OnOffControl = "cr50_uart_capture"
)

// CCD levels
const (
	Open   string = "open"
	Lock   string = "lock"
	Unlock string = "unlock"
)

// TestlabState contains possible ccd testlab states.
type TestlabState string

// Possible testlab states.
const (
	Enable  TestlabState = "on"
	Disable TestlabState = "off"
)

// CCDCap contains possible CCD capabilities.
type CCDCap string

// CCD capabilities.
const (
	UartGscRxAPTx   CCDCap = "UartGscRxAPTx"
	UartGscTxAPRx   CCDCap = "UartGscTxAPRx"
	UartGscRxECTx   CCDCap = "UartGscRxECTx"
	UartGscTxECRx   CCDCap = "UartGscTxECRx"
	FlashAP         CCDCap = "FlashAP"
	FlashEC         CCDCap = "FlashEC"
	OverrideWP      CCDCap = "OverrideWP"
	RebootECAP      CCDCap = "RebootECAP"
	GscFullConsole  CCDCap = "GscFullConsole"
	UnlockNoReboot  CCDCap = "UnlockNoReboot"
	UnlockNoShortPP CCDCap = "UnlockNoShortPP"
	OpenNoTPMWipe   CCDCap = "OpenNoTPMWipe"
	OpenNoLongPP    CCDCap = "OpenNoLongPP"
	BatteryBypassPP CCDCap = "BatteryBypassPP"
	Unused          CCDCap = "Unused"
	I2C             CCDCap = "I2C"
	FlashRead       CCDCap = "FlashRead"
	OpenNoDevMode   CCDCap = "OpenNoDevMode"
	OpenFromUSB     CCDCap = "OpenFromUSB"
	OverrideBatt    CCDCap = "OverrideBatt"
	APROCheckVC     CCDCap = "APROCheckVC"
)

// CCDCapState contains possible states for a CCD capability.
type CCDCapState string

// CCD capability states
const (
	CapDefault      CCDCapState = "Default"
	CapAlways       CCDCapState = "Always"
	CapUnlessLocked CCDCapState = "UnlessLocked"
	CapIfOpened     CCDCapState = "IfOpened"
)

// RunCR50Command runs the given command on the Cr50 on the device.
func (s *Servo) RunCR50Command(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, CR50UARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing CR50 UART Regexp")
	}
	return s.SetString(ctx, CR50UARTCmd, cmd)
}

// RunCR50CommandGetOutput runs the given command on the Cr50 on the device and returns the output matching patterns.
func (s *Servo) RunCR50CommandGetOutput(ctx context.Context, cmd string, patterns []string) ([][]string, error) {
	err := s.SetStringList(ctx, CR50UARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting CR50UARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, CR50UARTRegexp, "None")
	err = s.SetString(ctx, CR50UARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting CR50UARTCmd to %s", cmd)
	}
	iList, err := s.GetStringList(ctx, CR50UARTCmd)
	if err != nil {
		return nil, errors.Wrap(err, "decoding string list")
	}
	return ConvertToStringArrayArray(ctx, iList)
}

// CheckGSCBootMode verifies that the boot mode as reported by GSC's ec_comm command is as expected.
func (s *Servo) CheckGSCBootMode(ctx context.Context, expectedMode string) error {
	output, err := s.RunCR50CommandGetOutput(ctx, "ec_comm", []string{`boot_mode\s*:\s*(\S+)\s`})
	if err != nil {
		return errors.Wrap(err, "failed to get boot mode")
	}
	if output[0][1] != expectedMode {
		return errors.Wrapf(err, "incorrect boot mode, got %q want %q", output[0][1], expectedMode)
	}
	return nil
}

// SetTestlab will perform the required power button presses to disable or enable CCD testlab mode.
func (s *Servo) SetTestlab(ctx context.Context, option TestlabState) error {
	// Verify CCD is open.
	regExpCcdOpen := `State:\s*Opened`
	if _, err := s.RunCR50CommandGetOutput(ctx, "ccd", []string{regExpCcdOpen}); err != nil {
		return errors.Wrap(err, "ccd is not open")
	}

	// Verify there is a servo micro or C2D2 connected.
	hasMicroOrC2D2, err := s.PreferDebugHeader(ctx)
	if err != nil {
		return errors.Wrap(err, "verifying the preferred debug header")
	}
	if !hasMicroOrC2D2 {
		return errors.New("no micro-servo or C2D2 found: manual procedure is required to modify testlab state")
	}

	testing.ContextLogf(ctx, "Setting testlab to %q", option)
	if err := s.RunCR50Command(ctx, "ccd testlab "+string(option)); err != nil {
		return errors.Wrapf(err, "failed setting testlab to %q", option)
	}

	// Waiting 1 second before starting the power pressing sequence.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait 1 second before the power pressing sequence")
	}

	// Press the power button for up to 20 seconds, and space a 1 second
	// interval between these presses. The Cr50 console doesn't care about
	// extra presses in between. As long as all the required presses are hit,
	// testlab state would change.
	testing.ContextLog(ctx, "Starting power button presses")
	ppTimeout := 20 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testlab, err := s.GetString(ctx, CR50Testlab)
		if err != nil {
			return errors.Wrap(err, "failed to get cr50_testlab")
		}
		if testlab == string(option) {
			return nil
		}

		if err := s.KeypressWithDuration(ctx, PowerKey, DurPress); err != nil {
			return errors.Wrap(err, "failed to press power button")
		}
		return errors.Errorf("testlab has not been set to %q", option)
	}, &testing.PollOptions{Timeout: ppTimeout, Interval: 1 * time.Second}); err != nil {
		return err
	}

	return nil
}

/*
GetCCDCapability will return the current state of a specific CCD capability.
Possible states are:
	0 = Default
	1 = Always
	2 = UnlessLocked
	3 = IfOpened
It will also return "Y" if the capability is accessible, and "-" otherwise.
*/
func (s *Servo) GetCCDCapability(ctx context.Context, capability CCDCap) (int, string, error) {
	re := `(` + string(capability) + `)\s*(\w|\W)\s*\d`
	var out [][]string
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err = s.RunCR50CommandGetOutput(ctx, "ccd", []string{re})
		if err != nil {
			return errors.Wrap(err, "failed to get capability state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 2 * time.Second}); err != nil {
		return 0, "", err
	}
	splitout := strings.Split(out[0][0], " ")
	state, err := strconv.Atoi(splitout[len(splitout)-1])
	if err != nil {
		return 0, "", errors.Wrapf(err, "unable to tell %s state", capability)
	}
	return state, splitout[len(splitout)-2], nil
}

/*
SetCCDCapability will try to set a CCD capability to a specific state.
Possible states are:
	Default
	Always
	UnlessLocked
	IfOpened
*/
func (s *Servo) SetCCDCapability(ctx context.Context, capabilities map[CCDCap]CCDCapState) error {
	// Information about CCD states is usually returned in the form of
	// '[CapabilityName|Y|1]'. Create a map to match each capability
	// state with the corresponding integer value.
	capMap := map[CCDCapState]int{
		CapDefault:      0,
		CapAlways:       1,
		CapUnlessLocked: 2,
		CapIfOpened:     3,
	}

	for capability, state := range capabilities {
		// Ensure the state we want is set.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			cmd := fmt.Sprintf("ccd set %s %s", capability, state)
			if err := s.RunCR50Command(ctx, cmd); err != nil {
				return errors.Wrapf(err, "failed to send command %q to cr50", cmd)
			}

			currState, _, err := s.GetCCDCapability(ctx, capability)
			if err != nil {
				return errors.Wrapf(err, "failed to get current %q state", capability)
			}

			if currState != capMap[state] {
				return errors.Errorf("got state %v, but expected %v", currState, capMap[state])
			}
			return nil
		}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 10 * time.Second}); err != nil {
			return errors.Wrapf(err, "failed to set %s to %s", capability, state)
		}
	}
	return nil
}

// Wrapper function to that runs a GSC command and ensures all Python regex
// patterns appear at least once in the response. This function will throw a
// pretty printed error otherwise.
func (s *Servo) CheckGSCCommandOutput(ctx context.Context, cmd string, regexs []string) error {
	matches, err := s.RunCR50CommandGetOutput(ctx, cmd, regexs)
	if err != nil {
		return errors.Wrap(err, "Failed to run `"+cmd+"` on GSC, expected regex patterns = {"+strings.Join(regexs, ",")+"}")
	}
	if len(matches) == 0 {
		// NOTE: I've never seen this case occur since `servod` will throw an
		// XML error if no matches are found
		return errors.New("Failed to get regex matches = {" + strings.Join(regexs, ",") + "} for `" + cmd + "`")
	}
	return nil
}

// Lock the CCD console by sending a GSC command
func (s *Servo) LockCCD(ctx context.Context) error {
	if err := s.CheckGSCCommandOutput(ctx, "ccd lock", []string{`CCD locked`}); err != nil {
		return errors.Wrap(err, "Failed to run 'ccd lock' on GSC: ")
	}
	return nil
}
