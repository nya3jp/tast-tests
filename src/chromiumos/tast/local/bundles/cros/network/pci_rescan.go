// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PCIRescan,
		Desc:         "Makes sure that the wireless interface can recover automatically if removed",
		Contacts:     []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"iwlwifi_rescan"},
	})
}

func findWirelessInterface() (string, error) {
	typeList := []string{"wlan", "mlan"}
	ifaceList, err := getInterfaceList()
	if err != nil {
		return "", err
	}
	for _, pref := range typeList {
		for _, iface := range ifaceList {
			if strings.HasPrefix(iface, pref) {
				return iface, nil
			}
		}
	}
	return "", errors.New("could not find an interface")
}

func getInterfaceList() ([]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	toRet := make([]string, len(files), len(files))
	for i, file := range files {
		toRet[i] = file.Name()
	}
	return toRet, nil
}

func getPCIE(iface string) (string, error) {
	deviceStr := fmt.Sprintf("/sys/class/net/%s/device", iface)
	temp, err := filepath.EvalSymlinks(deviceStr)
	if err != nil {
		return "", err
	}
	return filepath.Base(temp), nil
}

func PCIRescan(ctx context.Context, s *testing.State) {
	iface, err := findWirelessInterface()
	if err != nil {
		s.Fatal("Could not find valid wireless interface")
	}
	pcieID, err := getPCIE(iface)
	if err != nil {
		s.Fatal("Could not get pcieID: ", err)
	}

	rescanFile := fmt.Sprintf("/sys/class/net/%s/device/driver/module/parameters/remove_when_gone", iface)
	out, err := testexec.CommandContext(ctx, "cat", rescanFile).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Could not read rescan file: ", err)
	} else if string(out) != "Y\n" {
		s.Fatalf("wifi rescan should be enabled, current mode is %s", string(out))
	}
	driverPath := fmt.Sprintf("/sys/class/net/%s/device/remove", iface)
	driverRealPath, err := filepath.EvalSymlinks(driverPath)

	if err := ioutil.WriteFile(driverRealPath, []byte("1"), 0200); err != nil {
		s.Logf("Could not remove driver, %s: %s", iface, err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "lspci").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}

		// Match PCIE ID suffix
		f := func(c rune) bool {
			return unicode.IsSpace(c)
		}
		words := strings.FieldsFunc(string(out), f)
		for _, word := range words {
			if strings.HasSuffix(pcieID, word) {
				return nil
			}
		}

		return errors.New("device not visible in lspci")

	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Device did not recover: ", err)
	}
}
