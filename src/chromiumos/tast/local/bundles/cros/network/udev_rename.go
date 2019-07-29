// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
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
	intf, device, driver string
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

func restartInterface(ctx context.Context) error {
	iface, err := findInterface()
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}

	// We lose connectivity briefly. Tell recover_duts not to worry.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lock the check network hook")
	}
	defer unlock()

	deviceStr := fmt.Sprintf("/sys/class/net/%s/device", iface)
	fullLink, err := filepath.EvalSymlinks(deviceStr)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on payload %s", deviceStr)
	}
	// The payload being written to the file is the PCIE address gleamed
	// from the baseDir of the fullLink
	payload := filepath.Base(fullLink)

	// The driver path is the directory where we can bind and release the device.
	driverPath := deviceStr + "/driver"
	path, err := filepath.EvalSymlinks(driverPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on path %s", driverPath)
	}
	// writeToFile writes a payload to a file with a base directory of path.
	writeToFile := func(payload, path, file string) error {
		f, err := os.OpenFile(path+file, os.O_WRONLY, 0200)
		if err != nil {
			return errors.Wrap(err, "could not open file")
		}
		defer f.Close()
		// Need to pipe to write-only file
		w := bufio.NewWriter(f)
		fmt.Fprintf(w, filepath.Base(payload))
		w.Flush()
		return nil
	}

	if err := writeToFile(payload, path, "/unbind"); err != nil {
		return errors.Wrapf(err, "could not unbind driver, %s", iface)
	}
	if err := writeToFile(payload, path, "/bind"); err != nil {
		return errors.Wrapf(err, "could not bind driver, %s", iface)
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
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
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
