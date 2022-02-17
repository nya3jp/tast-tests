// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CheckHpsError returns an error in case the HPS has seen an internal error.
func CheckHpsError(hctx *HpsContext) error {
	value, err := GetRegisterValue(hctx, "6")
	if err != nil {
		// This could happen right after `hps mcu reset`, don't want to fail the test.
		return nil
	}
	if value != 0 {
		if hctx.FailOnHPSErrorRegister {
			return errors.Errorf("error register is not zero (0x%x)", value)
		}
	}
	return nil
}

// GetNumberOfPresenceOps return an increasing number of presence ops the HPS has executed.
// It's not guaranteed to start at 0 after power-cycle and is only valid if HPS is in presence mode.
func GetNumberOfPresenceOps(hctx *HpsContext) (uint16, error) {
	return GetRegisterValue(hctx, "13")
}

// DecodePresenceResult decodes a result of a presence operation:
// it's positive when a person is detected, and negative when no person is detected.
func DecodePresenceResult(fullValue uint16) int {
	value := int(fullValue & 0xff)
	if value < 0x80 {
		return value
	}
	return value - 256
}

// GetPresenceResult returns the decoded result of the last presence operation.
func GetPresenceResult(hctx *HpsContext, reg string) (int, error) {
	raw, err := GetRegisterValue(hctx, reg)
	if err != nil {
		return 0, err
	}
	return DecodePresenceResult(raw), nil
}

// GetRegisterValue returns the integer value from the HPS register.
func GetRegisterValue(hctx *HpsContext, register string) (uint16, error) {
	var output []byte
	var err error
	regex := regexp.MustCompile(`\b0x([0-9a-f]+)\b`)
	args := []string{"hps", hctx.Device, "status", register}
	if hctx.DutConn != nil {
		output, err = hctx.DutConn.CommandContext(hctx.Ctx, args[0], args[1:]...).CombinedOutput()
	} else {
		output, err = testexec.CommandContext(hctx.Ctx, args[0], args[1:]...).CombinedOutput()
	}

	if err != nil {
		return 0, err
	}
	result := regex.FindStringSubmatch(strings.ToLower(string(output)))
	if len(result) < 2 {
		return 0, errors.Errorf("Hex number pattern not found in %q (%q)", string(output), result)
	}
	value, err := strconv.ParseInt(result[1], 16, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "Conversion failed: %q", result[1])
	}
	if value > 0xffff {
		return 0, errors.Errorf("Register returned a value that doesn't fit in int16: 0x%x", value)
	}
	return uint16(value), nil
}

// RunBuiltinI2CDetect runs i2cdetect to check in what state the HPS is.
func RunBuiltinI2CDetect(hctx *HpsContext) error {
	outDir := hctx.OutDir
	cmdNum := &hctx.CmdNum
	// 0x51 is system bootloader: if it responds we can flash stage0.
	// Power-cycling gets us to address 51.
	// Brya p2 -- 15, Taeko -- 14
	if err := RunWithLog(hctx, outDir, "i2cdetect_0x51", cmdNum, "i2cdetect", "-y", "-r", "15", "0x51", "0x51"); err != nil {
		return err
	}

	// 0x30 is hps firmware: after starting hpsd.
	// Starting hpsd should get us from address 51 to address 30.
	if err := RunWithLog(hctx, outDir, "i2cdetect_0x30", cmdNum, "i2cdetect", "-y", "-r", "15", "0x30", "0x30"); err != nil {
		return err
	}

	return nil
}

// RunHpsTool runs the `hps device ...` command and writes both stdout and stderr to the log file.
func RunHpsTool(hctx *HpsContext, arg ...string) error {
	arg2 := append([]string{"hps", hctx.Device}, arg...)
	if err := RunWithLog(hctx, hctx.OutDir, "hps_tool", &hctx.CmdNum, arg2...); err != nil {
		return err
	}
	return CheckHpsError(hctx)
}

// RunWithLog runs the specified command and writes both stdout and stderr to the log file.
// cmdNum is used to automatically generate a consistent numeric prefix.
func RunWithLog(hctx *HpsContext, outDir, logFilename string, cmdNum *int, args ...string) error {
	ctx := hctx.Ctx
	if hctx.DutConn != nil {
		logFilename = fmt.Sprintf("%02d_%s.txt", *cmdNum, logFilename)
		cmd := hctx.DutConn.CommandContext(hctx.Ctx, args[0], args[1:]...)
		testing.ContextLog(ctx, cmd, " --> ", logFilename)
		*cmdNum++

		var output []byte
		var err error
		if output, err = cmd.Output(); err != nil {
			testing.ContextLog(ctx, "Error for the command: ", (err))
		}

		log, fileerr := os.OpenFile(filepath.Join(outDir, logFilename),
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if fileerr != nil {
			return errors.Wrapf(err, "cannot open log file %s", logFilename)
		}
		defer log.Close()
		if _, writeErr := log.WriteString(string(output)); writeErr != nil {
			return errors.Wrap(writeErr, "Wrote logcommand failed")
		}

		// Writing error if there is any
		if err != nil {
			if _, writeErr := log.WriteString(err.Error()); writeErr != nil {
				return errors.Wrap(writeErr, "Wrote logcommand failed")
			}
			return errors.Wrapf(err, "command failed, see %s for more details", logFilename)
		}

	} else {
		logFilename = fmt.Sprintf("%02d_%s.txt", *cmdNum, logFilename)
		cmd := testexec.CommandContext(hctx.Ctx, args[0], args[1:]...)
		testing.ContextLog(ctx, cmd, " --> ", logFilename)
		*cmdNum++

		log, err := os.OpenFile(filepath.Join(outDir, logFilename),
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return errors.Wrapf(err, "cannot open log file %s", logFilename)
		}
		defer log.Close()

		cmd.Stderr = log
		cmd.Stdout = log

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "command failed, see %s for more details", logFilename)
		}
	}

	return nil
}

// RunStatus runs the `hps status` command to help diagnose problems.
func RunStatus(hctx *HpsContext) error {
	if err := RunHpsTool(hctx, "status", "0", "11"); err != nil {
		return errors.Wrap(err, "error during getting status")
	}
	return nil
}
