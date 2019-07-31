// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PCIRescan,
		Desc:     "Checks that shill isn't respawning",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func findInterface() (string, error) {
	typeList := []string{"wlan", "mlan"}
	ifaceList, err := getInterfaceList()
	if err != nil {
		return "", err
	}
	for _, pref := range typeList {
		for _, intf := range *ifaceList {
			if strings.HasPrefix(intf, pref) {
				return intf, nil
			}
		}
	}
	return "", errors.New("could not find an interface")
}

func getInterfaceList() (*[]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, err
	}
	toRet := make([]string, len(files), len(files))
	for i, file := range files {
		toRet[i] = file.Name()
	}
	return &toRet, nil
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
	iface, err := findInterface()
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
	} else if string(out) != "1\n" {
		s.Fatalf("wifi rescan should be enabled, current mode is %s", string(out))
	}
	driverPath := fmt.Sprintf("/sys/class/net/%s/device/remove", iface)
	path, err := filepath.EvalSymlinks(driverPath)
	f, err := os.OpenFile(path, os.O_WRONLY, 0200)
	if err != nil {
		s.Fatal("Could not open file: ", err)
	}
	defer f.Close()
	// Need to pipe to write-only file
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "1")
	w.Flush()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "lspci").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		if strings.Contains(string(out), pcieID) {
			return nil
		}
		return errors.New("device not visible in lspci")

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Device did not recover: ", err)
	}
}
