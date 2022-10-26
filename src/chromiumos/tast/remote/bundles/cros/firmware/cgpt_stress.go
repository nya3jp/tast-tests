// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CgptStress,
		Desc:     "Test DUT shuts down and boots to ChromeOS over many iterations",
		Contacts: []string{"tij@google.com", "cros-fw-engprod@google.com"},
		// TODO(b/255617349): This test might be breaking duts, add "firmware_unstable" when fixed or ruled out.
		Attr:         []string{"group:firmware"},
		Vars:         []string{"firmware.CgptStressIters"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      100 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "dev_mode",
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

func CgptStress(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	numIters := 10
	if numItersStr, ok := s.Var("firmware.CgptStressIters"); ok {
		numItersInt, err := strconv.Atoi(numItersStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.CgptStressIters: got %q, expected int", numItersStr)
		} else {
			numIters = numItersInt
		}
	}

	// Backup current cgpt data.
	devData, err := getCgptInfo(ctx, h)
	if err != nil {
		s.Fatal("Failed to get initial cgpt data: ", err)
	}
	// Logs initial devData.
	s.Log("Current cgpt data:")
	for dev, data := range devData {
		s.Log(dev)
		for label, val := range data {
			s.Logf("\t%v: %v", label, val)
		}
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		currDevData, err := getCgptInfo(ctx, h)
		if err != nil {
			s.Fatal("Failed to get initial cgpt data: ", err)
		}
		// If current data doesn't match backup, set to backed up data.
		if !reflect.DeepEqual(currDevData, devData) {
			if err := setCgptInfo(ctx, h, devData); err != nil {
				s.Fatal("Failed to reset cgpt info: ", err)
			}

			ms, err := firmware.NewModeSwitcher(ctx, h)
			if err != nil {
				s.Fatal("Creating mode switcher: ", err)
			}
			if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
				s.Fatal("Failed to reboot: ", err)
			}
		}
	}(cleanupContext)

	// Set up kernel to boot from part a initially.
	if ok, err := checkRootPart(ctx, h, "a"); err != nil {
		s.Fatal("Failed to check root part: ", err)
	} else if !ok {
		// If not in root part a, set to a and reboot.
		if err := resetAndPrioritizeKernelPart(ctx, h, "a"); err != nil {
			s.Fatal("Failed to prioritize part a: ", err)
		}
	}

	for i := 0; i < numIters; i++ {
		s.Logf("Running iteration %d out of %d ", i+1, numIters)

		// Expect it to be in part a at start of iteration.
		if ok, err := checkRootPart(ctx, h, "a"); err != nil {
			s.Fatal("Failed to check root part: ", err)
		} else if !ok {
			s.Fatal("Expected root part a: ", err)
		}
		if err := resetAndPrioritizeKernelPart(ctx, h, "b"); err != nil {
			s.Fatal("Failed to prioritize kernel part b: ", err)
		}

		// Expect it to be in part b now.
		if ok, err := checkRootPart(ctx, h, "b"); err != nil {
			s.Fatal("Failed to check root part: ", err)
		} else if !ok {
			s.Fatal("Expected root part b: ", err)
		}
		if err := resetAndPrioritizeKernelPart(ctx, h, "a"); err != nil {
			s.Fatal("Failed to prioritize kernel part a: ", err)
		}

		// Expect it to be in part a again.
		if ok, err := checkRootPart(ctx, h, "a"); err != nil {
			s.Fatal("Failed to check root part: ", err)
		} else if !ok {
			s.Fatal("Expected root part a: ", err)
		}
	}
}

func checkRootPart(ctx context.Context, h *firmware.Helper, part string) (bool, error) {
	rootFsMap := map[string]string{"a": "3", "b": "5", "2": "3", "4": "5", "3": "3", "5": "5"}
	out, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s").Output(ssh.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to get rootdev")
	}
	devPath := strings.TrimSpace(string(out))
	// Gets single digit partition from device (eg. 3 from /dev/sda3 or 5 from /dev/mmcblk0p5).
	rootPart := string(devPath[len(devPath)-1:])
	testing.ContextLogf(ctx, "Current root part: %s; expected root part: %s", rootPart, rootFsMap[part])
	if rootFsMap[part] != rootPart {
		testing.ContextLogf(ctx, "Expected to be in part %v, but is currently in part %v", rootFsMap[part], rootPart)
		return false, nil
	}
	return true, nil
}

func resetAndPrioritizeKernelPart(ctx context.Context, h *firmware.Helper, part string) error {
	kernelMap := map[string]string{"a": "2", "b": "4", "2": "2", "4": "4", "3": "2", "5": "4"}

	out, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s", "-d").Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get rootdev")
	}
	rootDev := strings.TrimSpace(string(out))

	cmd := []string{"add", "-i", kernelMap["a"], "-P", "1", "-T", "0", "-S", "1", rootDev}
	out, err = h.DUT.Conn().CommandContext(ctx, "cgpt", cmd...).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to set cgpt values for kern a")
	}

	cmd = []string{"add", "-i", kernelMap["b"], "-P", "1", "-T", "0", "-S", "1", rootDev}
	out, err = h.DUT.Conn().CommandContext(ctx, "cgpt", cmd...).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to set cgpt values for kern b")
	}

	out, err = h.DUT.Conn().CommandContext(ctx, "cgpt", "prioritize", "-i", kernelMap[part], rootDev).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to prioritize cgpt part %q", part)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed creating mode switcher")
	}
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}
	return nil
}

func setCgptInfo(ctx context.Context, h *firmware.Helper, devData map[string]map[string]string) error {
	out, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s", "-d").Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get rootdev")
	}
	rootDev := strings.TrimSpace(string(out))

	// Modifies only the following settable properties: "priority", "tries", "successful".
	for partitionName, partition := range devData {
		if partition == nil {
			continue
		}
		cmd := []string{
			"add", "-i", partition["partition"],
			"-P", partition["priority"],
			"-T", partition["tries"],
			"-S", partition["successful"],
			rootDev,
		}
		if _, err := h.DUT.Conn().CommandContext(ctx, "cgpt", cmd...).Output(ssh.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to set cgpt values for %v", partitionName)
		}
	}
	return nil
}

func getCgptInfo(ctx context.Context, h *firmware.Helper) (map[string]map[string]string, error) {
	out, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s", "-d").Output(ssh.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rootdev")
	}
	rootDev := strings.TrimSpace(string(out))
	// Parse initial cgpt info.
	out, err = h.DUT.Conn().CommandContext(ctx, "cgpt", "show", rootDev).Output(ssh.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cgpt info")
	}
	devInfoStr := string(out)
	label := ""
	labelData := make(map[string]string)
	deviceData := make(map[string]map[string]string)
	for _, rawLine := range strings.Split(devInfoStr, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.Contains(line, "Label:") {
			if _, ok := deviceData[label]; label != "" && !ok {
				deviceData[label] = labelData
			}
			label = strings.Trim(strings.Split(line, "Label:")[1], `" `)
			partition := strings.Fields(line)[2]
			labelData = map[string]string{"partition": partition}
		} else if strings.Contains(line, ":") {
			nameVal := strings.Split(line, ":")
			name, val := nameVal[0], nameVal[1]
			if name != "Attr" {
				labelData[name] = strings.TrimSpace(val)
			} else {
				for _, attr := range strings.Split(strings.TrimSpace(val), " ") {
					newNameVal := strings.Split(attr, "=")
					labelData[strings.TrimSpace(newNameVal[0])] = strings.TrimSpace(newNameVal[1])
				}
			}
		}
	}

	// Return just the relevant cgpt data.
	return map[string]map[string]string{
		"KERN-A": deviceData["KERN-A"],
		"KERN-B": deviceData["KERN-B"],
	}, nil
}
