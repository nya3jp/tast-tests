// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     OutDirAfterReboot,
		Desc:     "Check OutDir exists after reboot",
		Contacts: []string{"seewaifu@chromium.org"},
	})
}

func writeTemp(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("test data\n")
	return err
}

func readTemp(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	return r.ReadString('\n')
}

// OutDirAfterReboot demonstrates how you'd use RPCDUT.
func OutDirAfterReboot(ctx context.Context, s *testing.State) {
	outDir := s.OutDir()
	testing.ContextLog(ctx, "outDir: ", outDir)
	filePath := filepath.Join(s.OutDir(), "test.log")
	if err := writeTemp(filePath); err != nil {
		s.Fatalf("Failed to write file %s: %v", filePath, err)
	}

	s.DUT().Reboot(ctx)

	testing.ContextLog(ctx, "outDir after reboot: ", s.OutDir())
	buf, err := readTemp(filePath)
	if err != nil {
		s.Fatalf("Failed to read file %s: %v", filePath, err)
	}
	testing.ContextLogf(ctx, "content of %s: %s", filePath, buf)

}
