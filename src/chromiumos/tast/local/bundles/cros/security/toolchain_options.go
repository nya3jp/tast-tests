// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"debug/elf"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/toolchain"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ToolchainOptions,
		Desc: "Verifies that system ELF executables were compiled with a hardened toolchain",
		Contacts: []string{
			"jorgelo@chromium.org",     // Security team
			"kathrelkeld@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"no_asan"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{
			{
				Val: toolchain.CheckNormal,
			},
			{
				Name: "allowlist",
				Val:  toolchain.CheckAllowlist,
			},
			{
				Name: "dlc",
				Val:  toolchain.CheckNormalWithDLCs,
			},
		},
	})
}

// Paths that will be pruned/ignored when searching for ELF files.
var prunePaths = []string{
	"/proc",
	"/dev",
	"/sys",
	"/mnt/stateful_partition",
	"/usr/local",
	// There are files in /home (e.g. encrypted files that look
	// like they have ELF headers) that cause false positives,
	// and since that's noexec anyways, it should be skipped.
	"/home",
	"/opt/google/containers",
	"/run/containers/android_*/root",
	// Linux perftools saves debug symbol cache under $HOME/.debug (crbug.com/1004817#c9).
	"/root/.debug",
	// Skip vdso .so files which are built together with the kernel without RELRO
	"/lib/modules/*/vdso",
	"/lib/modules/*/vdso32",
	// Skip /run/lacros, as the nacl binary is currently compiled with the Chrome toolchain.
	"/run/lacros",
}

// Allowed files for the BIND_NOW condition.
var nowAllowlist = []string{
	// FIXME: crbug.com/535032
	"/opt/google/chrome/nacl_helper_nonsfi",

	// Allowed in crbug.com/682434.
	"/usr/lib64/conntrack-tools/ct_helper_*.so",
	"/usr/lib/conntrack-tools/ct_helper_*.so",
	"/usr/sbin/nfct",
}

var relroAllowlist = []string{
	// FIXME: crbug.com/535032
	"/opt/google/chrome/nacl_helper_nonsfi",
}

var pieAllowlist []string

var textrelAllowlist []string

var stackAllowlist []string

var loadwxAllowlist []string

func loadDLCs(ctx context.Context) error {
	criticalDLCs := []string{"pita"}
	for _, dlc := range criticalDLCs {
		metadataPath := filepath.Join("/opt/google/dlc", dlc)
		// Load the DLC if metadata exists for it as it should be preloadable on test images.
		if _, err := os.Stat(metadataPath); err == nil {
			if err := testexec.CommandContext(ctx, "dlcservice_util", "--install", "--id="+dlc, "--omaha_url=").Run(testexec.DumpLogOnError); err != nil {
				return errors.Wrapf(err, "failed in loading %s DLC", dlc)
			}
			testing.ContextLog(ctx, "Successfully loaded DLC: ", dlc)
		}
	}
	return nil
}

func ToolchainOptions(ctx context.Context, s *testing.State) {
	mode := s.Param().(toolchain.CheckMode)
	if mode == toolchain.CheckNormalWithDLCs {
		// TODO(crbug.com/1077056): Remove once DLC provisioning is in place for F20.
		// Also make this generic to any DLCs to be checked against, without exhausting the loopback devices.
		if err := loadDLCs(ctx); err != nil {
			s.Error("Loading DLC failed: ", err)
		}
	}

	var conds []*toolchain.ELFCondition

	// Condition: Verify non-static binaries have BIND_NOW in dynamic section.
	conds = append(conds, toolchain.NewELFCondition(toolchain.NowVerify, nowAllowlist))

	// Condition: Verify non-static binaries have RELRO program header.
	conds = append(conds, toolchain.NewELFCondition(toolchain.RelroVerify, relroAllowlist))

	// Condition: Verify non-static binaries are dynamic (built PIE).
	conds = append(conds, toolchain.NewELFCondition(toolchain.PieVerify, pieAllowlist))

	// Condition: Verify dynamic ELFs don't include TEXTRELs.
	conds = append(conds, toolchain.NewELFCondition(toolchain.TextrelVerify, textrelAllowlist))

	// Condition: Verify all binaries have non-exec STACK program header.
	conds = append(conds, toolchain.NewELFCondition(toolchain.StackVerify, stackAllowlist))

	// Condition: Verify no binaries have W+X LOAD program headers.
	conds = append(conds, toolchain.NewELFCondition(toolchain.LoadwxVerify, loadwxAllowlist))

	err := filepath.Walk("/", func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			// The file has been removed after listed in the parent directory.
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip matches from prunePaths.
			for _, pp := range prunePaths {
				m, err := filepath.Match(pp, path)
				if err != nil {
					s.Fatalf("Could not match %v to %v", path, pp)
				}
				if m {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			// Skip non-regular and non-executable files.
			return nil
		}

		ef, err := elf.Open(path)
		if err != nil {
			// Skip files that are not valid ELF files.
			return nil
		}
		defer ef.Close()

		// Run all defined condition checks on this ELF file.
		for _, c := range conds {
			if err := c.CheckAndFilter(path, ef, mode); err != nil {
				s.Error("Condition failure: ", err)

				// Print details of the offending file for debugging.
				if fi, err := os.Stat(path); err != nil {
					s.Logf("File info: %s: failed to stat: %v", path, err)
				} else {
					var ctime time.Time
					if st, ok := fi.Sys().(*syscall.Stat_t); ok {
						ctime = time.Unix(int64(st.Ctim.Sec), int64(st.Ctim.Nsec))
					}
					s.Logf("File info: %s: size=%d, mode=%o, ctime=%s, mtime=%s",
						path, fi.Size(), fi.Mode(), ctime.Format(time.RFC3339Nano),
						fi.ModTime().Format(time.RFC3339Nano))
				}
			}
		}
		return nil
	})
	if err != nil {
		s.Error("Error walking path: ", err)
	}
}
