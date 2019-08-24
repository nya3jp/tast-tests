// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"debug/elf"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
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
		Attr:         []string{"informational"},
		// TODO: Review commented out whitelist files before promoting this
		// test.  Uncomment any which are still causing failures and delete
		// any which are not.
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
}

// File match strings which will be ignored when searching for ELF files.
var ignoreMatches = []string{
	"libstdc++.so.*",
	"libgcc_s.so.*",
}

// Whitelisted files for the BIND_NOW condition.
var nowWhitelist = []string{
	// FIXME: crbug.com/535032
	"/opt/google/chrome/nacl_helper_nonsfi",
	// FIXME: crbug.com/887869
	"/opt/intel/fw_parser",
	//"/sbin/insmod.static",
	//"/usr/bin/cvt",
	//"/usr/bin/gtf",
	//"/usr/bin/synclient",
	//"/usr/bin/syndaemon",

	// Whitelisted in crbug.com/682434.
	"/usr/lib64/conntrack-tools/ct_helper_amanda.so",
	"/usr/lib64/conntrack-tools/ct_helper_dhcpv6.so",
	"/usr/lib64/conntrack-tools/ct_helper_ftp.so",
	"/usr/lib64/conntrack-tools/ct_helper_mdns.so",
	"/usr/lib64/conntrack-tools/ct_helper_rpc.so",
	"/usr/lib64/conntrack-tools/ct_helper_sane.so",
	"/usr/lib64/conntrack-tools/ct_helper_ssdp.so",
	"/usr/lib64/conntrack-tools/ct_helper_tftp.so",
	"/usr/lib64/conntrack-tools/ct_helper_tns.so",
	"/usr/lib/conntrack-tools/ct_helper_amanda.so",
	"/usr/lib/conntrack-tools/ct_helper_ftp.so",
	"/usr/lib/conntrack-tools/ct_helper_mdns.so",
	"/usr/lib/conntrack-tools/ct_helper_dhcpv6.so",
	"/usr/lib/conntrack-tools/ct_helper_tftp.so",
	"/usr/lib/conntrack-tools/ct_helper_tns.so",
	"/usr/lib/conntrack-tools/ct_helper_rpc.so",
	"/usr/lib/conntrack-tools/ct_helper_sane.so",
	"/usr/lib/conntrack-tools/ct_helper_ssdp.so",
	"/usr/sbin/nfct",
}

var relroWhitelist = []string{
	"/home/autotest/tests/logging_UserCrash/src/crasher_nobreakpad",
	// FIXME: crbug.com/535032
	"/opt/google/chrome/nacl_helper_nonsfi",
	//"/opt/google/chrome/pepper/libnetflixidd.so",
}

var pieWhitelist = []string{
	"/home/autotest/tests/logging_UserCrash/src/crasher_nobreakpad",
	//"/usr/bin/getent",
	// FIXME: Remove this after Intel fixes their parser. crbug.com/887869
	"/opt/intel/fw_parser",
	//"/opt/google/talkplugin/GoogleTalkPlugin",
}

var textrelWhitelist = []string{
	// For nyan boards. b/35583075
	//"/usr/lib/libGLdispatch.so.0",
}

var stackWhitelist = []string{
	// FIXME: Remove this after Intel fixes their parser. crbug.com/887869
	"/opt/intel/fw_parser",

	//"/usr/bin/gobi-fw",
}

var loadwxWhitelist []string

var libgccWhitelist = []string{
	"/opt/google/chrome/nacl_helper",

	// FIXME: Remove this after Intel fixes their parser. crbug.com/887869
	"/opt/intel/fw_parser",

	// Files from flash player.
	"/opt/google/chrome/libwidevinecdm.so",
	"/opt/google/chrome/pepper/libpepflashplayer.so",
	// Prebuilt hdcp driver binary from Intel.
	"/usr/sbin/hdcpd",
	// Prebuilt binaries installed by Intel Camera HAL on kabylake boards.
	"/usr/lib64/libia_ltm.so",
	"/usr/lib64/libSkyCamAIC.so",
	"/usr/lib64/libSkyCamAICKBL.so",

	// FIXME: Remove after Rust code is built with compiler-rt. crbug.com/814480
	"/usr/sbin/mosys",
	"/usr/bin/crosvm",
	"/usr/bin/memd",
	"/usr/bin/9s",
	"/usr/bin/crostini_client",
	"/usr/lib64/libqcow_utils.so",
	"/usr/lib64/libcrosvm_plugin.so",

	// FIXME: Remove after mesa is fixed to not need libgcc_s. crbug.com/808264
	"/usr/lib/dri/kms_swrast_dri.so",
	"/usr/lib/dri/swrast_dri.so",
	// Same for betty.
	"/usr/lib64/dri/kms_swrast_dri.so",
	"/usr/lib64/dri/swrast_dri.so",
	"/usr/lib64/dri/virtio_gpu_dri.so",
}

var libstdcWhitelist = []string{
	// Flash player
	"/opt/google/chrome/libwidevinecdm.so",
	"/opt/google/chrome/pepper/libpepflashplayer.so",

	// FIXME: Remove this after Intel fixes their parser. crbug.com/887869
	"/opt/intel/fw_parser",

	// Prebuilt hdcp driver binary from Intel.
	"/usr/sbin/hdcpd",
	// Prebuilt binaries installed by Intel Camera HAL on kabylake boards.
	"/usr/lib64/libbroxton_ia_pal.so",
	"/usr/lib64/libia_ltm.so",
	"/usr/lib64/libSkyCamAIC.so",
	"/usr/lib64/libSkyCamAICKBL.so",
	// Part of prebuilt driver binary used in Tegra boards.
	"/usr/lib/libnvmmlite_video.so",
	// Whitelisted in b/73422412.
	"/opt/google/rta/rtanalytics_main",
}

// elfCondition is a specific condition which is verified against all
// not-skipped ELF files.
type elfCondition struct {
	verify    func(ef *elf.File) error
	whitelist map[string]struct{} // Set of literal paths to be skipped.
}

// newELFCondition takes a verification function and a list of literal paths
// to whitelist for that condition and returns a new elfCondition.
func newELFCondition(verify func(ef *elf.File) error, w []string) *elfCondition {
	set := make(map[string]struct{})
	for _, path := range w {
		set[path] = struct{}{}
	}
	return &elfCondition{verify, set}
}

// checkAndFilter takes in a file and checks it against an elfCondition,
// returning an error if the file is not whitelisted.
func (ec *elfCondition) checkAndFilter(path string, ef *elf.File) error {
	if _, ok := ec.whitelist[path]; ok {
		return nil
	}
	if err := ec.verify(ef); err != nil {
		return errors.Wrap(err, path)
	}
	return nil
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
	var conds []*elfCondition

	// Condition: Verify non-static binaries have BIND_NOW in dynamic section.
	nowVerify := func(ef *elf.File) error {
		if elfIsStatic(ef) {
			return nil
		}
		dtFlags, err := findDynTagValue(ef, elf.DT_FLAGS)
		if err == nil && elf.DynFlag(dtFlags)&elf.DF_BIND_NOW == elf.DF_BIND_NOW {
			return nil
		}
		_, err = findDynTagValue(ef, elf.DT_BIND_NOW)
		return err
	}
	conds = append(conds, newELFCondition(nowVerify, nowWhitelist))

	// Condition: Verify non-static binaries have RELRO program header.
	const progTypeGnuRelro = elf.ProgType(0x6474e552)
	relroVerify := func(ef *elf.File) error {
		if elfIsStatic(ef) {
			return nil
		}
		for _, p := range ef.Progs {
			if p.Type == progTypeGnuRelro {
				return nil
			}
		}
		return errors.New("no GNU_RELRO program header found")
	}
	conds = append(conds, newELFCondition(relroVerify, relroWhitelist))

	// Condition: Verify non-static binaries are dynamic (built PIE).
	pieVerify := func(ef *elf.File) error {
		if elfIsStatic(ef) {
			return nil
		}
		for _, p := range ef.Progs {
			if p.Type == elf.PT_DYNAMIC {
				return nil
			}
		}
		return errors.New("non-static file did not have PT_DYNAMIC tag")
	}
	conds = append(conds, newELFCondition(pieVerify, pieWhitelist))

	// Condition: Verify dynamic ELFs don't include TEXTRELs.
	textrelVerify := func(ef *elf.File) error {
		if elfIsStatic(ef) {
			return nil
		}
		dtFlags, err := findDynTagValue(ef, elf.DT_FLAGS)
		if err != nil && elf.DynFlag(dtFlags)&elf.DF_TEXTREL == elf.DF_TEXTREL {
			return errors.New("TEXTREL flag found")
		}
		return nil
	}
	conds = append(conds, newELFCondition(textrelVerify, textrelWhitelist))

	// Condition: Verify all binaries have non-exec STACK program header.
	const progTypeGnuStack = elf.ProgType(0x6474e551)
	stackVerify := func(ef *elf.File) error {
		for _, p := range ef.Progs {
			if p.Type == progTypeGnuStack {
				if p.Flags&elf.PF_X == elf.PF_X {
					return errors.New("exec GNU_STACK program header found")
				}
				return nil
			}
		}
		return nil // Ignore if GNU_STACK is not found.
	}
	conds = append(conds, newELFCondition(stackVerify, stackWhitelist))

	// Condition: Verify no binaries have W+X LOAD program headers.
	loadwxVerify := func(ef *elf.File) error {
		const progFlagWX = elf.PF_X | elf.PF_W
		for _, p := range ef.Progs {
			if p.Type == elf.PT_LOAD {
				if p.Flags&progFlagWX == progFlagWX {
					return errors.New("LOAD was both writable and executable")
				}
				return nil
			}
		}
		return nil // Ignore if LOAD is not found.
	}
	conds = append(conds, newELFCondition(loadwxVerify, loadwxWhitelist))

	verifyNotLinked := func(pattern string) func(ef *elf.File) error {
		return func(ef *elf.File) error {
			strs, err := ef.DynString(elf.DT_NEEDED)
			if err != nil {
				return nil
			}
			for _, str := range strs {
				if m, _ := filepath.Match(pattern, str); m {
					return errors.Errorf("file linked with %s", str)
				}
			}
			return nil
		}
	}

	// Condition: Verify all binaries are not linked with libgcc_s.so.
	libgccVerify := verifyNotLinked("libgcc_s.so*")
	conds = append(conds, newELFCondition(libgccVerify, libgccWhitelist))

	// Condition: Verify all binaries are not linked with libstdc++.so.
	libstdcVerify := verifyNotLinked("libstdc++.so*")
	conds = append(conds, newELFCondition(libstdcVerify, libstdcWhitelist))

	err := filepath.Walk("/", func(path string, info os.FileInfo, err error) error {
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
			if err := c.checkAndFilter(path, ef); err != nil {
				s.Error("Condition failure: ", err)
			}
		}
		return nil
	})
	if err != nil {
		s.Error("Error walking path: ", err)
	}
}
