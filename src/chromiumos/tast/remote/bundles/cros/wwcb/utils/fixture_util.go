// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils used to do some component excution function.
package utils

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
	// "chromiumos/tast/remote/bundles/cros/wwcb/lib/go.bug.st/serial"
)

var (
	fixtureUID = map[string]string{
		"AUS19129_C01_01": "1912901",
		"AUS19129_C01_02": "1912902",

		"AHS20079_A00_01": "2007901",
		"AHS20079_A00_02": "2007902",

		"AUS20019_D00_01": "2001901",
		"AUS20019_D00_02": "2001902",
		"AUS20019_D00_03": "2001903",
		"AUS20019_D00_04": "2001904",
		"AUS20019_D00_05": "2001905",

		"ADT21090_B00_01": "2109001",
		"ADT21090_B00_02": "2109002",

		"XXRJ45SW_X00_01": "j45sw01",
	}

	fixtureIsDisplay = map[string]bool{
		"1912901": true,
		"1912902": true,
		"2007901": true,
		"2007902": true,
		"2001901": false,
		"2001902": false,
		"2001903": false,
		"2001904": false,
		"2001905": false,
		"2109001": true,
		"2109002": true,
		"j45sw01": false,
	}

	fixtureCmd = map[string]map[string]string{
		"1912901": {"off": "0", "on": "1", "flip": "2"},
		"1912902": {"off": "0", "on": "1", "flip": "2"},
		"2007901": {"off": "2", "on": "1"},
		"2007902": {"off": "2", "on": "1"},
		"2001901": {"off": "2", "on": "1"},
		"2001902": {"off": "2", "on": "1"},
		"2001903": {"off": "2", "on": "1"},
		"2001904": {"off": "2", "on": "1"},
		"2001905": {"off": "2", "on": "1"},
		"2109001": {"off": "2", "on": "1"},
		"2109002": {"off": "2", "on": "1"},
		"j45sw01": {"off": "2", "on": "1"},
	}

	// key:text fixture uid , value:usb port id
	fixtureOnline = make(map[string]string)
)

// InitFixture initializes fixtures.
func InitFixture(ctx context.Context) error {
	ports, err := serial.GetPortsList()
	if err != nil {
		return errors.Wrap(err, "failed to get port list")
	}

	if len(ports) == 0 {
		return errors.New("no serial ports found")
	}

	// In order to prevent switch fixture function not in time.
	testing.Sleep(ctx, 3000)

	// Print the list of detected ports.
	for _, port := range ports {
		// Open the first serial port detected at 9600bps N81.
		mode := &serial.Mode{
			BaudRate: 9600,
		}

		usbPort, err := serial.Open(port, mode)
		if err != nil {
			return errors.New(err)
		}

		var t time.Duration = 3 * time.Second
		usbPort.SetReadTimeout(t)

		_, err = usbPort.Write([]byte("i"))
		if err != nil {
			return errors.New(err)
		}

		// Read and print the response.
		buff := make([]byte, 1000)
		for {
			// Reads up to 100 bytes.
			n, err := usbPort.Read(buff)

			if err != nil {
				return errors.New(err)
				break
			}

			if n == 0 {
				fmt.Println("\nEOF")
				break
			}

			mString := string(buff[:n])
			mString = strings.Replace(mString, "\r", "", -1)
			res := strings.Split(mString, "\n")

			for _, s := range res {
				if len(s) > 22 && strings.Count(s[0:15], "_") == 2 {
					serial := s[0:15]
					fixtureID := fmt.Sprintf("test fixture id: %s => port: %s \n", serial, port)
					testing.ContextLog(ctx, fixtureID)

					uid, found := fixtureUID[serial]

					if found {
						fixtureOnline[uid] = port
					} else {
						return errors.Wrapf(err, "serial %s is not in fixture list", serial)
					}
				}
			}
		}
		usbPort.Close()
	}
}

// ControlFixture is for control fixture.
func ControlFixture(ctx context.Context, uid, cmd string) error {
	s := fmt.Sprintf("Fixture '%s' set '%s' ", uid, cmd)
	testing.ContextLog(ctx, s)

	port, found := fixtureOnline[uid]

	if !found {
		s := fmt.Sprintf("uid %s is not in online fixture list.", uid)
		return errors.New(s)
	}

	mode := &serial.Mode{
		BaudRate: 9600,
	}

	usbPort, err := serial.Open(port, mode)
	if err != nil {
		return errors.New(err)
	}

	var t time.Duration = 3 * time.Second
	usbPort.SetReadTimeout(t)

	cmdNum := fixtureCmd[uid][cmd]

	_, err = usbPort.Write([]byte(cmdNum))
	if err != nil {
		return errors.New(err)
	}

	usbPort.Close()

	// sleep for test fixture wake up.
	t = 7 * time.Second

	// In order to prevent switch fixture function not in time.
	testing.Sleep(ctx, t)
}

// OpenAllFixture is for open all fixture.
func OpenAllFixture(ctx context.Context) {
	for uid := range fixtureOnline {
		ControlFixture(ctx, uid, "on")
	}
}

// CloseAllFixture is for close all fixture.
func CloseAllFixture(ctx context.Context) {
	for uid := range fixtureOnline {
		ControlFixture(ctx, uid, "off")
	}
}

// PrintAllFixture is for print all fixture on context log.
func PrintAllFixture(ctx context.Context) {
	for uid, port := range fixtureOnline {
		s := fmt.Sprintf("print all fixture id: %s => port: %s \n", uid, port)
		testing.ContextLog(ctx, s)
	}
}
