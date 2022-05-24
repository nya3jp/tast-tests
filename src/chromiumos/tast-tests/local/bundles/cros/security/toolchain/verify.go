// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package toolchain contains support code for the security.ToolchainOptions test.
package toolchain

import (
	"debug/elf"
	"path/filepath"

	"chromiumos/tast/errors"
)

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

// NowVerify Condition: Verify non-static binaries have BIND_NOW in dynamic section.
func NowVerify(ef *elf.File) error {
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

// RelroVerify Condition: Verify non-static binaries have RELRO program header.
func RelroVerify(ef *elf.File) error {
	if elfIsStatic(ef) {
		return nil
	}
	const progTypeGnuRelro = elf.ProgType(0x6474e552)
	for _, p := range ef.Progs {
		if p.Type == progTypeGnuRelro {
			return nil
		}
	}
	return errors.New("no GNU_RELRO program header found")
}

// PieVerify Condition: Verify non-static binaries are dynamic (built PIE).
func PieVerify(ef *elf.File) error {
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

// TextrelVerify Condition: Verify dynamic ELFs don't include TEXTRELs.
func TextrelVerify(ef *elf.File) error {
	if elfIsStatic(ef) {
		return nil
	}
	dtFlags, err := findDynTagValue(ef, elf.DT_FLAGS)
	if err != nil && elf.DynFlag(dtFlags)&elf.DF_TEXTREL == elf.DF_TEXTREL {
		return errors.New("TEXTREL flag found")
	}
	return nil
}

// StackVerify Condition: Verify all binaries have non-exec STACK program header.
func StackVerify(ef *elf.File) error {
	const progTypeGnuStack = elf.ProgType(0x6474e551)
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

// LoadwxVerify Condition: Verify no binaries have W+X LOAD program headers.
func LoadwxVerify(ef *elf.File) error {
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

// CreateNotLinkedVerify Condition: Verify all binaries are not linked with |pattern|.
func CreateNotLinkedVerify(pattern string) func(ef *elf.File) error {
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
