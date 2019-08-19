// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gadget

import (
	"context"
	"runtime"
	"fmt"

	"chromiumos/tast/local/usb"
)

var (
	ErrBusy     = fmt.Errorf("gadget busy")
	ErrNotReady = fmt.Errorf("gadget not ready")
)

// Function interface for gadget function implementation.
type Function interface {
	Name() string
	Start(context.Context, ConfigFragment) error
	Stop() error
}

// Gadget provides primary interface for USB Device emulation
type Gadget struct {
	description   usb.DeviceInfo
	config       *Config
	functions   []Function
	plugged       bool
}

// NewGadget returns new instance of the USB Gadget.
func NewGadget(description usb.DeviceInfo) *Gadget {
	return &Gadget {
		plugged      : false,
		description  : description,
	}
}

func (g *Gadget) IsActive() bool {
	return g.config != nil
}

// Register gadget function and assign it to device config.
func (g *Gadget) Register(f Function) error {
	if g.IsActive() {
		return ErrBusy
	}
	g.functions = append(g.functions, f)
	return nil
}

// Start the USB gadget device including all registered functions.
func (g *Gadget) Start(ctx context.Context) (err error) {
	var config *Config

	if g.IsActive() {
		return ErrBusy
	}

	if config, err = NewTempConfig(); err != nil {
		return
	}

	check := func(result error) { if err == nil { err = result } }

	check(config.Set("idVendor", g.description.VendorId))
	check(config.Set("idProduct", g.description.ProductId))
	check(config.Set("bcdDevice", g.description.DeviceRev))
	check(config.Set("bcdUSB", g.description.UsbRev))
	check(config.Set("bDeviceClass", g.description.DeviceClass))
	check(config.Set("bDeviceSubClass", g.description.DeviceSubClass))
	check(config.Set("bDeviceProtocol", g.description.DeviceProtocol))
	check(config.SetString("product", g.description.Product))
	check(config.SetString("manufacturer", g.description.Manufacturer))
	check(config.SetString("serialnumber", g.description.SerialNumber))

	if err != nil {
		config.Remove()
		return
	}

	// single device configuration descriptor
	var c ConfigFragment

	c, err = config.Config("c.1")
	check(c.Set("MaxPower", 500))
	check(c.SetString("configuration", "c"))

	if err != nil {
		config.Remove()
		return
	}

	for i, f := range(g.functions) {
		if foo, e := config.Function(f.Name()); e != nil {
			err = e
		} else if e := c.Set(f.Name(), foo); e != nil {
			err = e
		} else if e := f.Start(ctx, foo); e != nil {
			err = e
		} else {
			continue
		}
		for _, f := range(g.functions[:i]) {
			f.Stop()
		}
		config.Remove()
		return
	}

	g.config = config
	runtime.SetFinalizer(g, func(g *Gadget) { g.Stop() })
	return
}

// Stop the USB gadget device including all registered functions.
func (g *Gadget) Stop() error {
	if !g.IsActive() {
		return nil
	}
	if g.plugged {
		g.Unbind()
	}
	runtime.SetFinalizer(g, nil)
	for _, f := range(g.functions) {
		if err := f.Stop(); err != nil {
			return err
		}
	}
	if err := g.config.Remove(); err != nil {
		return err
	}
	g.config = nil
	return nil
}

// Bind gadget instance to USB Device Controller
func (g *Gadget) Bind(udc usb.DevicePort) (err error) {
	if g.IsActive() {
		g.plugged = g.config.Set("UDC", udc.Name()) == nil
	} else {
		err = ErrNotReady
	}
	return
}

// Unbind removes gadget instance from UDC
func (g *Gadget) Unbind() {
	if g.plugged {
		g.config.Set("UDC", "")
		g.plugged = false
	}
}

// Id returns device vendor and product ID string.
func (g *Gadget) Id() string {
	return fmt.Sprintf("%04x:%04x", g.description.VendorId, g.description.ProductId)
}
