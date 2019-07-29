// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UdevRename,
		Desc:     "Verifies that network interfaces don't disappear completely after udev or the interface drivers are restarted",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

type iface struct {
	intf, driver string
}

func findDriver(ifname string) (string, error) {
	driver := fmt.Sprintf("/sys/class/net/%s/device/driver/module", ifname)
	if rl, err := os.Readlink(driver); err != nil {
		return "", errors.Wrap(err, "could not find driver")
	}
	return filepath.Base(rl), nil
}

func findInterface() (*iface, error) {
	typeList := []string{"wlan", "mlan", "eth"}
	ifaceList, err := getInterfaceList()
	if err != nil {
		return nil, err
	}
	for _, pref := range typeList {
		for _, intf := range *ifaceList {
			if strings.HasPrefix(intf, pref) {
				if res, err := findDriver(intf); err != nil {
					return nil, err
				} else if res != "" {
					return &iface{intf: intf, driver: res}, nil
				}
			}
		}
	}
	return nil, errors.New("could not find an interface")
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

func restartInterface(ctx context.Context) error {
	iface, err := findInterface()
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}
	if iface.driver == "iwlwifi" {
		err = testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", iface.driver).Run(testexec.DumpLogOnError)
	} else {
		err = testexec.CommandContext(ctx, "modprobe", "-r", iface.driver).Run(testexec.DumpLogOnError)
	}
	if err2 := testexec.CommandContext(ctx, "modprobe", iface.driver).Run(testexec.DumpLogOnError); err2 != nil {
		if err != nil {
			return errors.Wrap(err2, err.Error())
		}
		return err2
	}
	if err != nil {
		return err
	}
	return nil
}

func restartUdev(ctx context.Context) error {
	const service string = "udev"
	if exists := upstart.JobExists(ctx, service); !exists {
		return errors.Errorf("%s not running", service)
	}
	if err := upstart.StopJob(ctx, service); err != nil {
		return err
	}
	if err := upstart.StartJob(ctx, service); err != nil {
		return err
	}
	if exists := upstart.JobExists(ctx, service); !exists {
		return errors.Errorf("%s failed to stay running", service)
	}
	return nil
}

type udevExec func(ctx context.Context) error

func testUdevDeviceList(ctx context.Context, fn udevExec) error {
	iflistPre, err := getInterfaceList()
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		iflistPost, err := getInterfaceList()
		if err != nil {
			return err
		}
		if diff := cmp.Diff(iflistPre, iflistPost); diff == "" {
			return nil
		}
		return errors.Errorf("Interface changed %v != %v", iflistPre, iflistPost)
	}, &testing.PollOptions{Timeout: time.Second, Interval: 100 * time.Millisecond}); err != nil {
		return err
	}
	return nil
}

func UdevRename(ctx context.Context, s *testing.State) {
	if err := testUdevDeviceList(ctx, restartUdev); err != nil {
		s.Fatal(err.Error())
	}

	if err := testUdevDeviceList(ctx, restartInterface); err != nil {
		s.Fatal(err.Error())
	}
}
