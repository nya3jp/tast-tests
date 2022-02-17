// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// HasHpsContext is an interface for fixture values that contain a HpsContext instance. It allows
// retrieval of the underlying HpsContext object.
type HasHpsContext interface {
	HpsContext(ctx context.Context, outDir string) *HpsContext
}

// HpsContext stores common information passed around when calling hps tool and flashing the devboards.
type HpsContext struct {
	Ctx context.Context
	// Script to power-cycle p2 devboard.
	p2PowerCycleFilename string
	// Used as prefix for command logs.
	CmdNum     int
	deviceType DeviceType
	// String passed to the `hps` tool.
	Device string
	// Directory where the test results are stored.
	OutDir string
	// Ignore HPS error register errors when set to false.
	FailOnHPSErrorRegister bool

	DutConn *ssh.Conn
}

// NewHpsContext creates new HpsContext.
func NewHpsContext(ctx context.Context, p2PowerCycleFilename string, deviceType DeviceType, outDir string, dutConn *ssh.Conn) (*HpsContext, error) {
	var device string
	switch deviceType {
	case DeviceTypeDevboard:
		device = devboardDevice
	case DeviceTypeBuiltin:
		device = builtinDevice
	default:
		return nil, errors.New("Unhandled deviceType")
	}
	return &HpsContext{
		Ctx:                    ctx,
		p2PowerCycleFilename:   p2PowerCycleFilename,
		CmdNum:                 1,
		deviceType:             deviceType,
		Device:                 device,
		OutDir:                 outDir,
		FailOnHPSErrorRegister: true,
		DutConn:                dutConn,
	}, nil
}

// HpsContext returns the HpsContext instance.
// It implements the HasChrome interface.
func (hctx *HpsContext) HpsContext(ctx context.Context, outDir string) *HpsContext {
	return &HpsContext{
		Ctx:                    ctx,
		p2PowerCycleFilename:   hctx.p2PowerCycleFilename,
		CmdNum:                 1,
		deviceType:             hctx.deviceType,
		Device:                 hctx.Device,
		OutDir:                 outDir,
		FailOnHPSErrorRegister: true,
	}
}

// PowerCycle power-cycles the HPS, and makes sure it's usable afterwards.
func (hctx *HpsContext) PowerCycle() error {
	switch hctx.deviceType {
	case DeviceTypeDevboard:
		// TODO(mblsha): Package pyftdi in an ebuild. Workaround to start gathering power data as early as possible.
		if err := RunWithLog(hctx, hctx.OutDir, "pip", &hctx.CmdNum, "pip"); err != nil {
			// Install pip and pytfdi.
			if err := RunWithLog(hctx, hctx.OutDir, "wget", &hctx.CmdNum,
				"wget", "https://bootstrap.pypa.io/get-pip.py", "-O", "/tmp/get-pip.py"); err != nil {
				return err
			}

			if err := RunWithLog(hctx, hctx.OutDir, "get-pip", &hctx.CmdNum, "python", "/tmp/get-pip.py"); err != nil {
				return err
			}

			if err := RunWithLog(hctx, hctx.OutDir, "pip_install_pyftdi", &hctx.CmdNum,
				"pip", "install", "pyftdi"); err != nil {
				return err
			}
		}

		if err := RunWithLog(hctx, hctx.OutDir, "proto2-power-cycle", &hctx.CmdNum,
			"python", hctx.p2PowerCycleFilename); err != nil {
			return err
		}

		if err := testing.Sleep(hctx.Ctx, firmwareVerificationDelay); err != nil {
			return errors.Wrap(err, "sleep failed")
		}

		if err := unbindUsbHid(hctx); err != nil {
			return err
		}
	case DeviceTypeBuiltin:
		// TODO(mblsha): Rely on hpsd to flash the firmware after http://b/202680181 is fixed.
		// This might fail if the hpsd wasn't running, ignore this error.
		RunWithLog(hctx, hctx.OutDir, "stop_hpsd", &hctx.CmdNum, "stop", "hpsd")

		outDir := hctx.OutDir
		cmdNum := &hctx.CmdNum
		if err := RunWithLog(hctx, outDir, "gpio_0", cmdNum, "gpioset", "0", "327=0"); err != nil {
			return err
		}

		// If the power is off for two short a time, the MCU won't have properly lost power and weird stuff can happen.
		if err := testing.Sleep(hctx.Ctx, time.Second*1); err != nil {
			return errors.Wrap(err, "sleep failed")
		}

		if err := RunWithLog(hctx, outDir, "gpio_1", cmdNum, "gpioset", "0", "327=1"); err != nil {
			return err
		}

		if err := testing.Sleep(hctx.Ctx, firmwareVerificationDelay); err != nil {
			return errors.Wrap(err, "sleep failed")
		}

		if err := RunBuiltinI2CDetect(hctx); err != nil {
			return err
		}
	default:
		return errors.New("Unhandled deviceType")
	}
	return nil
}
