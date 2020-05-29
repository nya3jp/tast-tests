// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Microcode,
		Desc: "Checks that compatible CPU microcode is built into the kernel",
		Contacts: []string{
			"mnissler@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Microcode(ctx context.Context, s *testing.State) {
	vmlinuz := readKernelImage(ctx, s)
	vmlinux := unpackKernelImage(vmlinuz, s)
	fwmap := extractBuiltinFirmware(vmlinux, s)

	cpuinfo := readCPUInfo(s)
	for _, cpu := range cpuinfo {
		if cpu["vendor_id"] != "GenuineIntel" {
			continue
		}

		id := fmt.Sprintf("%02x-%02x-%02x",
			toUint64(s, cpu["cpu family"]),
			toUint64(s, cpu["model"]),
			toUint64(s, cpu["stepping"]))
		fwname := fmt.Sprintf("intel-ucode/%s", id)

		ucode, ok := fwmap[fwname]
		if !ok {
			// Test failure here indicates that no microcode that matches the processor
			// ID is bundled in the kernel. Make sure that cros-kernel2.eclass includes
			// the appropriate microcode in CONFIG_EXTRA_FIRMWARE.
			s.Errorf("No built-in microcode for id %s", id)
			continue
		}

		idx := toUint64(s, cpu["processor"])
		pf := readSysfsCPUVal(idx, "microcode/processor_flags", s)
		rev := readSysfsCPUVal(idx, "microcode/version", s)

		s.Logf("CPU %d id %s pf %#02x rev %#02x", idx, id, pf, rev)

		ucodeRev := uint32(0)
		for _, header := range parseUCode(ucode, s) {
			// This is a simplified check for whether microcode is compatible. In
			// particular, it doesn't take into account the trailing header that may
			// list alternative and processor ids flags. Logic to check that can be
			// added if we ever need it.
			if (uint64(header.Pf) & pf) != 0 {
				if header.Rev > ucodeRev {
					ucodeRev = header.Rev
				}
			}
		}

		if ucodeRev == 0 {
			// Test failure here indicates that while microcode for the processor ID is
			// present, it doesn't appear compatible with the CPU. To fix this, make
			// sure the correct microcode gets built into the kernel image by
			// cros-kernel2.eclass. If correct microcode appears to be present and get
			// loaded correctly, this may be because a bug in the test code, e.g.
			// because of disagreement between test test code and the kernel a microcode
			// image is compatible.
			s.Errorf("Microcode for id %s not compatible with pf %#02x", id, pf)
		}

		// Tolerate cases where the running microcode is a different, but more later
		// revision than the bundled one. This can happen if the CPU ships with updated
		// microcode or if firmware includes fresher microcode than the kernel.
		if rev < uint64(ucodeRev) {
			// If we fail here, then we found microcode in the kernel image that looks
			// compatible in the but it is not running on the CPU. This could be caused
			// by a corrupt microcode binary.
			s.Errorf("Microcode rev mismatch: %#02x < %#02x", rev, ucodeRev)
		}
	}
}

func readSysfsCPUVal(idx uint64, name string, s *testing.State) uint64 {
	path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/%s", idx, name)

	b, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", path, err)
	}

	return toUint64(s, string(b))
}

type ucodeHeader struct {
	Hdrver    uint32
	Rev       uint32
	Date      uint32
	Sig       uint32
	Cksum     uint32
	Ldrver    uint32
	Pf        uint32
	Datasize  uint32
	Totalsize uint32
	Reserved  [3]uint32
}

func parseUCode(ucode []byte, s *testing.State) []ucodeHeader {
	var r []ucodeHeader
	rdr := bytes.NewReader(ucode)
	for {
		var hdr ucodeHeader
		err := binary.Read(rdr, binary.LittleEndian, &hdr)
		if err != nil {
			if err == io.EOF {
				break
			}
			s.Fatal("Failed to read microcode header: ", err)
		}

		_, err = rdr.Seek(int64(hdr.Totalsize)-int64(binary.Size(hdr)), io.SeekCurrent)
		if err != nil {
			s.Fatal("Failed to skip microcode data: ", err)
		}

		r = append(r, hdr)
	}

	return r
}

func readCPUInfo(s *testing.State) []map[string]string {
	cpuinfo, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		s.Fatal("Failed to read cpuinfo: ", err)
	}

	cpus := strings.Split(string(cpuinfo), "\n\n")
	r := make([]map[string]string, len(cpus))
	for i, cpu := range cpus {
		r[i] = map[string]string{}
		for _, line := range strings.Split(cpu, "\n") {
			c := strings.Split(line, ":")
			if len(c) == 2 {
				key := strings.TrimSpace(c[0])
				value := strings.TrimSpace(c[1])
				r[i][key] = value
			}
		}
	}

	return r
}

func toUint64(s *testing.State, str string) uint64 {
	v, err := strconv.ParseInt(strings.TrimSpace(str), 0, 64)
	if err != nil {
		s.Fatalf("Failed to convert %q to uint64: %v", v, err)
	}

	return uint64(v)
}

type fwentry struct {
	Name uint64
	Addr uint64
	Size uint64
}

// extractBuiltinFirmware parses a kernel image and extracts a map of built-in firmware blobs. The
// .builtin_fw section in the image contains (name, location, size) triples, where name and location
// reference memory in the .rodata section. See the build commands for CONFIG_EXTRA_FIRMWARE in the
// kernel tree for details.
func extractBuiltinFirmware(vmlinux []byte, s *testing.State) map[string][]byte {
	f, err := elf.NewFile(bytes.NewReader(vmlinux))
	if err != nil {
		s.Fatal("Failed to parse kernel image: ", err)
	}
	defer f.Close()

	fws := f.Section(".builtin_fw")
	if fws == nil {
		s.Fatal("No .builtin_fw section")
	}
	fwsr := fws.Open()

	ros := f.Section(".rodata")
	if ros == nil {
		s.Fatal("No .rodata section")
	}
	ror := ros.Open()

	m := make(map[string][]byte)
	for {
		var fwe fwentry
		err := binary.Read(fwsr, binary.LittleEndian, &fwe)
		if err != nil {
			if err == io.EOF {
				break
			}
			s.Fatal("Failed to read built-in firmware section entry: ", err)
		}

		_, err = ror.Seek(int64(fwe.Name-ros.Addr), io.SeekStart)
		if err != nil {
			s.Fatal("Failed to seek to name addr: ", err)
		}

		name, err := bufio.NewReader(ror).ReadString(0)
		if err != nil {
			s.Fatal("Failed to read built-in firmware section entry name: ", err)
		}
		name = strings.TrimRight(name, "\x00")

		_, err = ror.Seek(int64(fwe.Addr-ros.Addr), io.SeekStart)
		if err != nil {
			s.Fatal("Failed to seek to built-in firmware address: ", err)
		}

		fw := make([]byte, fwe.Size)
		n, err := ror.Read(fw)
		if err != nil {
			s.Fatal("Failed to read built-in firmware data: ", err)
		}
		if uint64(n) != fwe.Size {
			s.Fatal("Short built-in firmware read")
		}

		s.Logf("Built-in firmware %s (%d bytes)", name, len(fw))

		m[name] = fw
	}

	return m
}

// readKernelImage obtains the kernel image from the booted kernel partition.
func readKernelImage(ctx context.Context, s *testing.State) []byte {
	dev := getKernelPartition(ctx, s)

	vmlinux, err := testexec.CommandContext(ctx, "futility", "vbutil_kernel", "--get-vmlinuz", dev, "--vmlinuz-out", "/dev/stdout").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to dump vmlinuz: ", err)
	}

	return vmlinux
}

// Matches the partition number on a block device name.
var rePartition = regexp.MustCompile("[0-9]+$")

// getKernelPartition determines the booted kernel partition device name from rootdev output.
func getKernelPartition(ctx context.Context, s *testing.State) string {
	dev, err := testexec.CommandContext(ctx, "rootdev", "-s").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute rootdev: ", err)
	}

	return rePartition.ReplaceAllStringFunc(
		strings.TrimSpace(string(dev)),
		func(match string) string { return strconv.FormatUint(toUint64(s, match)-1, 10) })
}

// unpackKernelImage decompresses gzip-compressed kernel image. See scripts/extract-vmlinux in the
// kernel source tree for reference.
func unpackKernelImage(vmlinuz []byte, s *testing.State) []byte {
	// Search for gzip header.
	offset := bytes.Index(vmlinuz, []byte{0x1f, 0x8b, 0x08})
	if offset == -1 {
		s.Fatal("Failed to locate gzip header")
	}

	zr, err := gzip.NewReader(bytes.NewReader(vmlinuz[offset:]))
	if err != nil {
		s.Fatal("Failed to create gzip reader: ", err)
	}
	// This is required so the GZIP parser doesn't try to interpret the trailing data in the
	// image as another stream and fails on that.
	zr.Multistream(false)

	vmlinux, err := ioutil.ReadAll(zr)
	if err != nil {
		s.Fatal("Failed to decompress: ", err)
	}

	return vmlinux
}
