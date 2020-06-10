// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
				Name:      "allowlist",
				Val:       toolchain.CheckAllowlist,
				ExtraAttr: []string{"informational"},
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
}

// File match strings which will be ignored when searching for ELF files.
var ignoreMatches = []string{
	"libstdc++.so.*",
	"libgcc_s.so.*",
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

var libgccAllowlist = []string{
	"/opt/google/chrome/nacl_helper",

	// Files from flash player.
	"/opt/google/chrome/pepper/libpepflashplayer.so",
	// Prebuilt hdcp driver binary from Intel.
	"/usr/sbin/hdcpd",
	// Prebuilt binaries installed by Intel Camera HAL on kabylake boards.
	"/usr/lib64/libia_ltm.so",
	"/usr/lib64/libSkyCamAIC.so",
	"/usr/lib64/libSkyCamAICKBL.so",

	// FIXME: Remove after mesa is fixed to not need libgcc_s. crbug.com/808264
	"/usr/lib/dri/kms_swrast_dri.so",
	"/usr/lib/dri/swrast_dri.so",
	// Same for betty.
	"/usr/lib64/dri/kms_swrast_dri.so",
	"/usr/lib64/dri/swrast_dri.so",
	"/usr/lib64/dri/virtio_gpu_dri.so",

	// Prebuilt binaries installed by Mediatek Camera HAL on kukui boards.
	// See b/140535983.
	"/usr/lib/lib3a.*.so",
	"/usr/lib/libcamalgo.*.so",
}

var libstdcAllowlist = []string{
	// Flash player
	"/opt/google/chrome/pepper/libpepflashplayer.so",

	// Prebuilt hdcp driver binary from Intel.
	"/usr/sbin/hdcpd",
	// Prebuilt binaries installed by Intel Camera HAL on kabylake boards.
	"/usr/lib64/libbroxton_ia_pal.so",
	"/usr/lib64/libia_ltm.so",
	"/usr/lib64/libSkyCamAIC.so",
	"/usr/lib64/libSkyCamAICKBL.so",
	// Part of prebuilt driver binary used in Tegra boards.
	"/usr/lib/libnvmmlite_video.so",
	// Allowed in b/73422412.
	"/opt/google/rta/rtanalytics_main",
}

// elfIsStatic returns whether the ELF file is statically linked.
func elfIsStatic(ef *elf.File) bool {
	return ef.FileHeader.Type != elf.ET_DYN
}

// findDynTagValue returns value of the given elf.DynTag in the ELF file's
// dynamic section, or an error if no such tag is found.
func findDynTagValue(ef *elf.File, tag elf.DynTag) (uint64, error) {
	ds := ef.SectionByType(elf.SHT_DYNAMIC)
	if ds == nil {
		return 0, errors.New("no Dynamic Section found")
	}
	d, err := ds.Data()
	if err != nil {
		return 0, errors.New("no Data read from Dynamic Section")
	}
	for len(d) > 0 {
		var t elf.DynTag
		var v uint64
		switch ef.Class {
		case elf.ELFCLASS32:
			t = elf.DynTag(ef.ByteOrder.Uint32(d[0:4]))
			v = uint64(ef.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case elf.ELFCLASS64:
			t = elf.DynTag(ef.ByteOrder.Uint64(d[0:8]))
			v = ef.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == tag {
			return v, nil
		}
	}
	return 0, errors.Errorf("%s not found in Dynamic Section", tag)
}

func ToolchainOptions(ctx context.Context, s *testing.State) {
	mode := s.Param().(toolchain.CheckMode)

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

	// Condition: Verify all binaries are not linked with libgcc_s.so.
	libgccVerify := toolchain.CreateNotLinkedVerify("libgcc_s.so*")
	conds = append(conds, toolchain.NewELFCondition(libgccVerify, libgccAllowlist))

	// Condition: Verify all binaries are not linked with libstdc++.so.
	libstdcVerify := toolchain.CreateNotLinkedVerify("libstdc++.so*")
	conds = append(conds, toolchain.NewELFCondition(libstdcVerify, libstdcAllowlist))

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
		// Skip ignored files from ignoreMatches.
		for _, im := range ignoreMatches {
			m, err := filepath.Match(im, info.Name())
			if err != nil {
				s.Fatalf("Could not match %v to %v", path, im)
			}
			if m {
				return nil
			}
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
