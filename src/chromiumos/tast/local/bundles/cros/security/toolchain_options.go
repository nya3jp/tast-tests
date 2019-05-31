// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"debug/elf"
	"os"
	"path/filepath"

	//"chromiumos/tast/local/testexec"
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
		Attr: []string{"informational"},
	})
}

// Paths that will be pruned/ignored when searching for ELF files
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
	"/opt/google/containers/android/rootfs/root/vendor",
	"/run/containers/android_*/root/vendor",
}

// File match strings which will be ignored when searching for ELF files
var ignoreMatches = []string{
	"libstdc++.so.*",
	"libgcc_s.so.*",
}

// Whitelisted files for the BIND_NOW condition
var nowWhitelist = []string{
	// crbug.com/570550
	"/opt/google/chrome/nacl_helper_nonsfi",
	// crbug.com/887869
	"/opt/intel/fw_parser",
	"/sbin/insmod.static",
	"/usr/bin/cvt",
	"/usr/bin/gtf",
	// crbug.com/341095
	"/usr/bin/intel-virtual-output",
	"/usr/bin/synclient",
	"/usr/bin/syndaemon",
	// crbug.com/682434
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

// elfCondition is the info about a specific condition which will be run against all not-skipped ELF files
type elfCondition struct {
	name      string
	verify    func(fh *elf.File) bool
	whitelist []string
	failures  []string
}

func newElfCondition(name string, check func(fh *elf.File) bool, whitelist []string) *elfCondition {
	return &elfCondition{
		name,
		check,
		whitelist,
		[]string{},
	}
}

// checkAndFilter takes in a file and checks it against an elfCondition, adding
// the file's path to the condition's failure list if it is an actual error.
func (ec *elfCondition) checkAndFilter(s *testing.State, path string, fh *elf.File) error {
	valid := ec.verify(fh)
	if !valid {
		// Ignore the failure if bad file matches whitelist
		for _, wElt := range ec.whitelist {
			m, err := filepath.Match(wElt, path)
			if err != nil {
				return err
			}
			if m == true {
				s.Log("Skipped elt from whitelist ", path)
				return nil
			}
		}
		// Add non-whitelisted bad path to failures slice
		ec.failures = append(ec.failures, path)
	}
	return nil
}

func isDynamic(fh *elf.File) bool {
	return fh.FileHeader.Type == elf.ET_DYN
}

func findDynTagValue(fh *elf.File, tag elf.DynTag) (uint64, bool) {
	ds := fh.SectionByType(elf.SHT_DYNAMIC)
	if ds == nil {
		return 0, false
	}
	d, err := ds.Data()
	if err != nil {
		return 0, false
	}
	for len(d) > 0 {
		var t elf.DynTag
		var v uint64
		switch fh.Class {
		case elf.ELFCLASS32:
			t = elf.DynTag(fh.ByteOrder.Uint32(d[0:4]))
			v = uint64(fh.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case elf.ELFCLASS64:
			t = elf.DynTag(fh.ByteOrder.Uint64(d[0:8]))
			v = fh.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == tag {
			return v, true
		}
	}
	return 0, false
}

func ToolchainOptions(ctx context.Context, s *testing.State) {
	conds := []*elfCondition{}

	// Condition: Verify non-static binaries have BIND_NOW in dynamic section
	nowVerify := func(fh *elf.File) bool {
		if isDynamic(fh) {
			dtFlag, valid := findDynTagValue(fh, elf.DT_FLAGS)
			if valid && (elf.DynFlag(dtFlag)&elf.DF_BIND_NOW == elf.DF_BIND_NOW) {
				return true
			}
			_, valid = findDynTagValue(fh, elf.DT_BIND_NOW)
			if valid {
				return true
			}
			return false

		}
		return true
	}
	conds = append(conds, newElfCondition("BIND_NOW", nowVerify, nowWhitelist))

	//TODO add hardfp option?
	//TODO Skip binaries built with Address Sanitizer as it is a separate testing tool.
	rootdir := "/" //TODO make this an option? Does anyone use the test this way?
	err := filepath.Walk(rootdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, pp := range prunePaths { // Skip pruned paths
				m, err := filepath.Match(pp, path)
				if err != nil {
					return errors.Wrapf(err, "Could not match %v to %v", path, pp)
				}
				if m {
					s.Log("Skipping ", path)
					return filepath.SkipDir
				}
			}
		} else if !info.Mode().IsRegular() || info.Mode()&1 != 1 { // Skip non-regular and non-executable files
			return nil
		} else {

			for _, im := range ignoreMatches { // Skip ignored files
				m, err := filepath.Match(im, info.Name())
				if err != nil {
					return errors.Wrapf(err, "Could not match %v to %v", path, im)
				}
				if m {
					return nil
				}
			}
			// All regular, executable files that are neither pruned or ignored are seen here
			fh, err := elf.Open(path)
			if err != nil { // Skip files that are not valid ELF files
				return nil
			}
			defer fh.Close()

			// Run all defined condition checks on this file.
			for _, c := range conds {
				err := c.checkAndFilter(s, path, fh)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		s.Error("Error walking path: ", err)
	}

	errCnt := 0
	for _, c := range conds {
		s.Log("Results for ", c.name)
		s.Log(c.failures)
		errCnt += len(c.failures)
	}
	if errCnt > 0 {
		//TODO make this more descriptive
		s.Fatalf("Found %v failures", errCnt)
	}
}
