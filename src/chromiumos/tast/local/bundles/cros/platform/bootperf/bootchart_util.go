// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootperf

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	bootchartKArg = "cros_bootchart"
)

// getRootPartition returns root partition index by running `rootdev -s` and
// converting the last digit to partition index.
// For example, "/dev/mmcblk0p3" corresponds to root partition index 2 that is
// used in modifying the kernel args.
func getRootPartition(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/rootdev", "-s").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed in getting the root partition")
	}

	// Sample output of "rootdev -s": /dev/nvme0p3, /dev/mmcblk0p3, /dev/sda3
	// Capture the ending numerical part, followed by a newline, of the output.
	// Reference: init of $ROOTDEV_PARTITION in src/platform/vboot_reference/scripts/image_signing/make_dev_ssd.sh.
	re := regexp.MustCompile(`.*\D(\d+)\s*$`)
	groups := re.FindStringSubmatch(string(out))
	if len(groups) != 2 {
		return "", errors.Errorf("failed to parse root partition from %s", out)
	}

	i, err := strconv.Atoi(groups[1])
	if err != nil {
		return "", errors.Errorf("failed to get partition index from %s", groups[1])
	}

	return strconv.Itoa(i - 1), nil
}

// editKernelArgs is a helper function for editing kernel args. Function |f|
// performs the editing action by transforming the content of saved config.
func editKernelArgs(ctx context.Context, f func([]byte) []byte) error {
	part, err := getRootPartition(ctx)
	if err != nil {
		return err
	}

	// Save the current boot config to |prefix|.|part| (make_dev_ssd.sh saves the content to a file named |prefix|.|part|).
	prefix := "/tmp/kargs" + uuid.New().String()
	err = testexec.CommandContext(ctx, "/usr/share/vboot/bin/make_dev_ssd.sh", "--save_config", prefix, "--partitions", part).Run()
	if err != nil {
		return errors.Wrap(err, "failed to save boot config")
	}

	savedKArgsFile := prefix + "." + part
	defer os.Remove(savedKArgsFile)

	savedKArgs, err := ioutil.ReadFile(savedKArgsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read saved kernel config")
	}

	// Transform the content.
	savedKArgs = f(savedKArgs)
	err = ioutil.WriteFile(savedKArgsFile, savedKArgs, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to edit saved kernel config")
	}

	err = testexec.CommandContext(ctx, "/usr/share/vboot/bin/make_dev_ssd.sh", "--set_config", prefix, "--partitions", part).Run()
	if err != nil {
		return errors.Wrap(err, "failed to set boot config")
	}

	return nil
}

// EnableBootchart enables bootchart by adding "cros_bootchart" to kernel
// arguments.
func EnableBootchart(ctx context.Context) error {
	if err := editKernelArgs(ctx, func(b []byte) []byte {
		s := string(b)
		if strings.Contains(s, bootchartKArg) {
			// Bootchart already enabled: leave the kernel args as is.
			return b
		}

		// Append "cros_bootchart" to kernel args.
		return []byte(s + " " + bootchartKArg)
	}); err != nil {
		return err
	}

	return nil
}

// DisableBootchart Disables bootchart by removing "cros_bootchart" from kernel
// arguments.
func DisableBootchart(ctx context.Context) error {
	if err := editKernelArgs(ctx, func(b []byte) []byte {
		s := string(b)
		if !strings.Contains(s, bootchartKArg) {
			// Bootchart already disabled: leave the kernel args as is.
			return b
		}

		// Remove "cros_bootchart" from kernel args.
		return []byte(strings.ReplaceAll(s, " "+bootchartKArg, ""))
	}); err != nil {
		return err
	}

	return nil
}
