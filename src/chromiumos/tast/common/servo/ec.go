// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// These are the EC Servo controls which can be get/set with a string value.
const (
	ECBoard            StringControl = "ec_board"
	ECSystemPowerState StringControl = "ec_system_powerstate"
	ECUARTCmd          StringControl = "ec_uart_cmd"
	ECUARTRegexp       StringControl = "ec_uart_regexp"
	ECUARTStream       StringControl = "ec_uart_stream"
	ECChip             StringControl = "ec_chip"
	ECFlashSize        StringControl = "ec_flash_size"
	DUTPDDataRole      StringControl = "dut_pd_data_role"
)

// These controls accept only "on" and "off" as values.
const (
	ECUARTCapture OnOffControl = "ec_uart_capture"
)

// Cmd constants for RunECCommand.
const (
	// Using with no additional arguments returns current backlight level
	// If additional int arg (0-100) provided, sets backlight to that level
	kbLight string = "kblight"
)

// Pattern expression for RunCommandGetOutput.
const (
	reKBBacklight        string = `Keyboard backlight: (\d+)\%`
	reCheckKBLight       string = `Keyboard backlight: \d+\%|Command 'kblight' not found or ambiguous.`
	reTabletmodeNotFound string = `Command 'tabletmode' not found or ambiguous`
	reBasestateNotFound  string = `Command 'basestate' not found or ambiguous`
	reTabletmodeStatus   string = `\[\S+ tablet mode (enabled|disabled)\]`
	reBasestateStatus    string = `\[\S+ base state: (attached|detached)\]`
	reBdStatus           string = `\[\S+ BD forced (connected|disconnected)\]`
	reLidAccel           string = `\[\S+ Lid Accel ODR:(?i)[^\n\r]*(?i)(1|0)\S+]`
)

// USBCDataRole is a USB-C data role.
type USBCDataRole string

// USB-C data roles.
const (
	// UFP is Upward facing partner, i.e. a peripheral. The servo should normally be in this role.
	UFP USBCDataRole = "UFP"
	// DFP is Downward facing partner, i.e. a host. The DUT should normally be in this role.
	DFP USBCDataRole = "DFP"
)

// KBMatrixPair is a struct to store key row and col for the kbpress cmd
type KBMatrixPair struct {
	row int
	col int
}

// KeyMatrix is a map that stores a row/col pair for each key using KBMatrixPair
// It's stored in order of appearance in a keyboard
var KeyMatrix = map[string]KBMatrixPair{
	"<esc>":       KBMatrixPair{1, 1},
	"<f1>":        KBMatrixPair{0, 2},
	"<f2>":        KBMatrixPair{3, 2},
	"<f3>":        KBMatrixPair{2, 2},
	"<f4>":        KBMatrixPair{1, 2},
	"<f5>":        KBMatrixPair{3, 4},
	"<f6>":        KBMatrixPair{2, 4},
	"<f7>":        KBMatrixPair{1, 4},
	"<f8>":        KBMatrixPair{2, 9},
	"<f9>":        KBMatrixPair{1, 9},
	"<f10>":       KBMatrixPair{0, 4},
	"`":           KBMatrixPair{3, 1},
	"1":           KBMatrixPair{6, 1},
	"2":           KBMatrixPair{6, 4},
	"3":           KBMatrixPair{6, 2},
	"4":           KBMatrixPair{6, 3},
	"5":           KBMatrixPair{3, 3},
	"6":           KBMatrixPair{3, 6},
	"7":           KBMatrixPair{6, 6},
	"8":           KBMatrixPair{6, 5},
	"9":           KBMatrixPair{6, 9},
	"0":           KBMatrixPair{6, 8},
	"-":           KBMatrixPair{3, 8},
	"=":           KBMatrixPair{0, 8},
	"<backspace>": KBMatrixPair{1, 11},
	"<tab>":       KBMatrixPair{2, 1},
	"q":           KBMatrixPair{7, 1},
	"w":           KBMatrixPair{7, 4},
	"e":           KBMatrixPair{7, 2},
	"r":           KBMatrixPair{7, 3},
	"t":           KBMatrixPair{2, 3},
	"y":           KBMatrixPair{2, 6},
	"u":           KBMatrixPair{7, 6},
	"i":           KBMatrixPair{7, 5},
	"o":           KBMatrixPair{7, 9},
	"p":           KBMatrixPair{7, 8},
	"[":           KBMatrixPair{2, 8},
	"]":           KBMatrixPair{2, 5},
	"\\":          KBMatrixPair{3, 11},
	"<search>":    KBMatrixPair{0, 1},
	"a":           KBMatrixPair{4, 1},
	"s":           KBMatrixPair{4, 4},
	"d":           KBMatrixPair{4, 2},
	"f":           KBMatrixPair{4, 3},
	"g":           KBMatrixPair{1, 3},
	"h":           KBMatrixPair{1, 6},
	"j":           KBMatrixPair{4, 6},
	"k":           KBMatrixPair{4, 5},
	"l":           KBMatrixPair{4, 9},
	";":           KBMatrixPair{4, 8},
	"'":           KBMatrixPair{1, 8},
	"<enter>":     KBMatrixPair{4, 11},
	"<shift_l>":   KBMatrixPair{5, 7},
	"z":           KBMatrixPair{5, 1},
	"x":           KBMatrixPair{5, 4},
	"c":           KBMatrixPair{5, 2},
	"v":           KBMatrixPair{5, 3},
	"b":           KBMatrixPair{0, 3},
	"n":           KBMatrixPair{0, 6},
	"m":           KBMatrixPair{5, 6},
	",":           KBMatrixPair{5, 5},
	".":           KBMatrixPair{5, 9},
	"/":           KBMatrixPair{5, 8},
	"<shift_r>":   KBMatrixPair{7, 7},
	"<ctrl_l>":    KBMatrixPair{2, 0},
	"<alt_l>":     KBMatrixPair{6, 10},
	" ":           KBMatrixPair{5, 1},
	"<alt_r>":     KBMatrixPair{0, 10},
	"<ctrl_r>":    KBMatrixPair{4, 0},
	"<left>":      KBMatrixPair{7, 12},
	"<up>":        KBMatrixPair{7, 11},
	"<down>":      KBMatrixPair{6, 11},
	"<right>":     KBMatrixPair{6, 12},
}

// HibernationOpt is an option for hibernating DUT.
type HibernationOpt string

// Available options for triggering hibernation.
const (
	// UseKeyboard uses keyboard shortcut for hibernating DUT: alt+vol_up+h.
	UseKeyboard HibernationOpt = "keyboard"
	// UseConsole uses the EC command `hibernate` to put DUT in hibernation.
	UseConsole HibernationOpt = "console"
)

// USBPdDualRoleValue contains a gettable/settable string accepted by
// the ec command, 'pd <port> dualrole'.
type USBPdDualRoleValue string

// These are acceptable states for the USB PD dual-role.
const (
	USBPdDualRoleOn     USBPdDualRoleValue = "on"
	USBPdDualRoleOff    USBPdDualRoleValue = "off"
	USBPdDualRoleFreeze USBPdDualRoleValue = "freeze"
	USBPdDualRoleSink   USBPdDualRoleValue = "force sink"
	USBPdDualRoleSource USBPdDualRoleValue = "force source"
)

// RunECCommand runs the given command on the EC on the device.
func (s *Servo) RunECCommand(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, ECUARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing EC UART Regexp")
	}
	return s.SetString(ctx, ECUARTCmd, cmd)
}

// RunECCommandGetOutput runs the given command on the EC on the device and returns the output matching patterns.
func (s *Servo) RunECCommandGetOutput(ctx context.Context, cmd string, patterns []string) ([][]string, error) {
	err := s.SetStringList(ctx, ECUARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, ECUARTRegexp, "None")
	err = s.SetString(ctx, ECUARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTCmd to %s", cmd)
	}
	iList, err := s.GetStringList(ctx, ECUARTCmd)
	if err != nil {
		return nil, errors.Wrap(err, "decoding string list")
	}
	return ConvertToStringArrayArray(ctx, iList)
}

// GetECSystemPowerState returns the power state, like "S0" or "G3"
func (s *Servo) GetECSystemPowerState(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECSystemPowerState)
}

// ECHibernate puts the EC into hibernation mode, after removing the servo watchdog for CCD if necessary.
func (s *Servo) ECHibernate(ctx context.Context, option HibernationOpt) error {
	if err := s.WatchdogRemove(ctx, WatchdogCCD); err != nil {
		return errors.Wrap(err, "failed to remove watchdog for ccd")
	}
	switch option {
	case "keyboard":
		if err := func(ctx context.Context) error {
			for _, targetKey := range []string{"<alt_l>", "<f10>", "h"} {
				row, col, err := s.GetKeyRowCol(targetKey)
				if err != nil {
					return errors.Wrapf(err, "failed to get key %s column and row", targetKey)
				}
				targetKeyName := targetKey
				targetKeyHold := fmt.Sprintf("kbpress %d %d 1", col, row)
				targetKeyRelease := fmt.Sprintf("kbpress %d %d 0", col, row)
				testing.ContextLogf(ctx, "Pressing and holding key %s", targetKey)
				if err := s.RunECCommand(ctx, targetKeyHold); err != nil {
					return errors.Wrapf(err, "failed to press and hold key %s", targetKey)
				}
				defer func(releaseKey, name string) error {
					testing.ContextLogf(ctx, "Releasing key %s", name)
					if err := s.RunECCommand(ctx, releaseKey); err != nil {
						return errors.Wrapf(err, "failed to release key %s", releaseKey)
					}
					return nil
				}(targetKeyRelease, targetKeyName)
			}
			return nil
		}(ctx); err != nil {
			return err
		}
	case "console":
		reHibernate := `\[\S+((?i)[^\n\r]*(?i)hibernating|hibernate)]`
		out, err := s.RunECCommandGetOutput(ctx, "hibernate", []string{reHibernate})
		if err != nil {
			return errors.Wrap(err, "failed to run EC command: hibernate")
		}
		if out != nil {
			testing.ContextLogf(ctx, "Found message: %q", out[0][0])
		} else {
			testing.ContextLog(ctx, "Did not find message about DUT hibernating")
		}
	}

	// Delay for a few seconds to allow proper propagation of the
	// hibernation command, prior to checking EC unresponsive.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	if err := s.CheckUnresponsiveEC(ctx); err != nil {
		return errors.Wrap(err, "while verifying whether EC is unresponsive after hibernating DUT")
	}
	return nil
}

// GetECFlashSize returns the size of EC in KB e.g. 512
func (s *Servo) GetECFlashSize(ctx context.Context) (int, error) {
	sizeStr, err := s.GetString(ctx, ECFlashSize)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get value for ec size")
	}
	// ECFlashSize method matches an int regex so Atoi should always work
	return strconv.Atoi(sizeStr)
}

// GetECChip returns the DUT chip e.g. "npcx_uut"
func (s *Servo) GetECChip(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECChip)
}

// SetDUTPDDataRole tries to find the port attached to the servo, and performs a data role swap if the role doesn't match `role`.
// Will fail if there is no chromeos EC.
func (s *Servo) SetDUTPDDataRole(ctx context.Context, role USBCDataRole) error {
	return s.SetString(ctx, DUTPDDataRole, string(role))
}

// GetKeyRowCol returns the key row and column for kbpress cmd
func (s *Servo) GetKeyRowCol(key string) (int, int, error) {
	pair, ok := KeyMatrix[key]
	if !ok {
		return 0, 0, errors.New("failed to find key in KeyMatrix map")
	}
	return pair.row, pair.col, nil

}

// ECPressKey simulates a keypress on the DUT from the servo using kbpress.
func (s *Servo) ECPressKey(ctx context.Context, key string) error {
	row, col, err := s.GetKeyRowCol(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get key %q in key matrix", key)
	}
	if err := s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 1", col, row)); err != nil {
		return errors.Wrapf(err, "failed to press key %q", key)
	}
	if err := s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 0", col, row)); err != nil {
		return errors.Wrapf(err, "failed to release key %q", key)
	}
	return nil
}

// SetKBBacklight sets the DUT keyboards backlight to the given value (0 - 100).
func (s *Servo) SetKBBacklight(ctx context.Context, percent int) error {
	testing.ContextLog(ctx, "Setting keyboard backlight to: ", percent)
	err := s.RunECCommand(ctx, fmt.Sprintf("%v %v", kbLight, percent))
	if err != nil {
		return errors.Wrapf(err, "running '%v %v' on DUT", kbLight, percent)
	}
	return nil
}

// GetKBBacklight gets the DUT keyboards backlight value in percent (0 - 100).
func (s *Servo) GetKBBacklight(ctx context.Context) (int, error) {
	testing.ContextLog(ctx, "Getting current keyboard backlight percent")
	out, err := s.RunECCommandGetOutput(ctx, kbLight, []string{reKBBacklight})
	if err != nil {
		return 0, errors.Wrapf(err, "running %v on DUT", kbLight)
	}
	return strconv.Atoi(out[0][1])
}

// HasKBBacklight checks if the DUT keyboards has backlight functionality.
func (s *Servo) HasKBBacklight(ctx context.Context) bool {
	testing.ContextLog(ctx, "Checking if DUT keyboard supports backlight")
	out, _ := s.RunECCommandGetOutput(ctx, kbLight, []string{reCheckKBLight})
	expMatch := regexp.MustCompile(reKBBacklight)
	match := expMatch.FindStringSubmatch(out[0][0])
	return match != nil
}

// CheckUnresponsiveEC verifies that EC console is unresponsive in situations such as
// hibernation and battery cutoff. Ignore null chars, sometimes the servo returns null
// when the EC is off.
func (s *Servo) CheckUnresponsiveEC(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := s.RunECCommandGetOutput(ctx, "version", []string{`[^\x00]+`})
		if err == nil {
			return errors.Errorf("EC is still active: got %v; expected error", out)
		}
		if !strings.Contains(err.Error(), "No data was sent from the pty") &&
			!strings.Contains(err.Error(), "EC: Timeout waiting for response.") &&
			!strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
			return errors.Wrap(err, "unexpected EC error")
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
}

// CheckAndRunTabletModeCommand checks if relevant EC commands exist and use them for setting tablet mode.
// For example, detachables use 'basestate (attach|detach)', and convertibles use 'tabletmode (on|off)'.
func (s *Servo) CheckAndRunTabletModeCommand(ctx context.Context, command string) (string, error) {
	// regular expressions.
	reStr := strings.Join([]string{reTabletmodeNotFound, reTabletmodeStatus,
		reBasestateNotFound, reBasestateStatus, reBdStatus, reLidAccel}, "|")
	checkTabletMode := fmt.Sprintf("%s%s%s", "(", reStr, ")")
	// Run EC command to check tablet mode setting.
	out, err := s.RunECCommandGetOutput(ctx, command, []string{checkTabletMode})
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %q", command)
	}
	tabletModeUnavailable := []*regexp.Regexp{regexp.MustCompile(reTabletmodeNotFound),
		regexp.MustCompile(reBasestateNotFound)}
	for _, v := range tabletModeUnavailable {
		if match := v.FindStringSubmatch(out[0][0]); match != nil {
			return "", errors.Errorf("device does not support tablet mode: %q", match)
		}
	}
	return string(out[0][1]), nil
}

// OpenCCD checks if a CCD connection exists, and then opens CCD if it's locked.
func (s *Servo) OpenCCD(ctx context.Context) error {
	if hasCCD, err := s.HasCCD(ctx); err != nil {
		return errors.Wrap(err, "while checking if servo has a CCD connection")
	} else if hasCCD {
		if val, err := s.GetString(ctx, GSCCCDLevel); err != nil {
			return errors.Wrap(err, "failed to get gsc_ccd_level")
		} else if val != Open {
			testing.ContextLogf(ctx, "CCD is not open, got %q. Attempting to unlock", val)
			if err := s.SetString(ctx, CR50Testlab, Open); err != nil {
				return errors.Wrap(err, "failed to unlock CCD")
			}
		}
		// For debugging purposes, log CCD state after unlocking CCD.
		checkedVal, err := s.GetString(ctx, GSCCCDLevel)
		if err != nil {
			return errors.Wrap(err, "failed to get gsc_ccd_level after unlocking CCD")
		}
		testing.ContextLogf(ctx, "CCD State: %q", checkedVal)
	}
	return nil
}

// CheckUSBPdStatus accepts a port ID and checks for the pd status of this port.
func (s *Servo) CheckUSBPdStatus(ctx context.Context, portID int, expectedStatus USBPdDualRoleValue) error {
	// Some DUTs use a different format in checking for pd: 'pd dualrole'.
	// This format was found on a few models, such as eve, nautilus, soraka,
	// kench, teemo, and sion. Check for the format that works. This list can be
	// further expanded using new test results.
	possibleCmds := []string{fmt.Sprintf("pd %d dualrole", portID), "pd dualrole"}
	matchList := []string{`dual-role toggling: ([^\n\r]*)`}
	checkPdCmd := func() string {
		for _, val := range possibleCmds {
			_, err := s.RunECCommandGetOutput(ctx, val, matchList)
			if err != nil {
				testing.ContextLog(ctx, err.Error())
				continue
			}
			return val
		}
		return ""
	}
	cmd := checkPdCmd()
	if cmd == "" {
		return errors.New("no command found to check for pd")
	}
	// Poll on pd cmd to avoid char lost.
	testing.ContextLog(ctx, "Checking for usb pd status")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := s.RunECCommandGetOutput(ctx, cmd, matchList)
		if err != nil {
			return errors.Wrapf(err, "failed to run cmd %s", cmd)
		}
		portStatus := USBPdDualRoleValue(out[0][1])
		if portStatus != expectedStatus {
			failStr := fmt.Sprintf("port %d dual-role: %s, expected: %s", portID, portStatus, expectedStatus)
			return errors.New(failStr)
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}
