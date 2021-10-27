// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	KBlight string = "kblight"
)

// Pattern expression for RunCommandGetOutput.
const (
	reKBBacklight   string = `Keyboard backlight: (\d+)\%`
	reNoKBBacklight string = `Command 'kblight' not found or ambiguous.`
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

// RunECCommand runs the given command on the EC on the device.
func (s *Servo) RunECCommand(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, ECUARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing EC UART Regexp")
	}
	return s.SetString(ctx, ECUARTCmd, cmd)
}

// RunECCommandGetOutput runs the given command on the EC on the device and returns the output matching patterns.
func (s *Servo) RunECCommandGetOutput(ctx context.Context, cmd string, patterns []string) ([]interface{}, error) {
	err := s.SetStringList(ctx, ECUARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, ECUARTRegexp, "None")
	err = s.SetString(ctx, ECUARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTCmd to %s", cmd)
	}
	return s.GetStringList(ctx, ECUARTCmd)
}

// GetECSystemPowerState returns the power state, like "S0" or "G3"
func (s *Servo) GetECSystemPowerState(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECSystemPowerState)
}

// ECHibernate puts the EC into hibernation mode, after removing the servo watchdog for CCD if necessary.
func (s *Servo) ECHibernate(ctx context.Context) error {
	// hibernateDelay is the time after the EC hibernate command where it still writes output
	const hibernateDelay = 1 * time.Second

	if err := s.WatchdogRemove(ctx, WatchdogCCD); err != nil {
		return errors.Wrap(err, "failed to remove watchdog for ccd")
	}
	if err := s.RunECCommand(ctx, "hibernate"); err != nil {
		return errors.Wrap(err, "failed to run EC command: hibernate")
	}
	testing.Sleep(ctx, hibernateDelay)

	// Verify the EC console is unresponsive, ignore null chars, sometimes the servo returns null when the EC is off.
	out, err := s.RunECCommandGetOutput(ctx, "version", []string{`[^\x00]+`})
	if err == nil {
		testing.ContextLogf(ctx, "Got %v expected error", out)
		return errors.New("EC is still active after hibernate")
	}
	if !strings.Contains(err.Error(), "No data was sent from the pty") &&
		!strings.Contains(err.Error(), "EC: Timeout waiting for response.") &&
		!strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
		return errors.Wrap(err, "unexpected EC error")
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
		return err
	}
	s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 1", col, row))
	s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 0", col, row))
	return nil
}

// SetKBBacklight sets the DUT keyboards backlight to the given value (0 - 100).
func (s *Servo) SetKBBacklight(ctx context.Context, percent int) error {
	testing.ContextLog(ctx, "Setting keyboard backlight to: ", percent)
	err := s.RunECCommand(ctx, fmt.Sprintf("%v %v", KBlight, percent))
	if err != nil {
		return errors.Wrapf(err, "running '%v %v' on DUT", KBlight, percent)
	}
	return nil
}

// GetKBBacklight gets the DUT keyboards backlight value in percent (0 - 100).
func (s *Servo) GetKBBacklight(ctx context.Context) (int, error) {
	testing.ContextLog(ctx, "Getting current keyboard backlight percent")
	out, err := s.RunECCommandGetOutput(ctx, KBlight, []string{reKBBacklight})
	if err != nil {
		return 0, errors.Wrapf(err, "running %v on DUT", KBlight)
	}
	return strconv.Atoi(out[0].([]interface{})[1].(string))
}

// HasKBBacklight checks if the DUT keyboards has backlight functionality.
func (s *Servo) HasKBBacklight(ctx context.Context) bool {
	testing.ContextLog(ctx, "Checking if DUT keyboard supports backlight")
	_, err := s.RunECCommandGetOutput(ctx, "kblight", []string{reNoKBBacklight})
	// If has backlight, command will time out, if no backlight, it will not have error.
	return err != nil
}
