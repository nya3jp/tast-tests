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
	"strconv"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Microcode,
		Desc: "Checks that suitable microcode is built into the kernel",
		Contacts: []string{
			"mnissler@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Microcode(ctx context.Context, s *testing.State) {
	fwmap := extractFirmware(s)

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
			s.Fatal("No built-in microcode")
		}

		// TODO: consider making sure that the firmware is available to the kernel by triggering a request via the test_firmware kernel module.

		idx := toUint64(s, cpu["processor"])
		pf := readSysfsCPUVal(idx, "microcode/processor_flags", s)
		rev := readSysfsCPUVal(idx, "microcode/version", s)
		ucodeRev := uint32(0)
		for _, header := range parseUCode(ucode, s) {
			if (uint64(header.Pf)&pf) != 0 && header.Rev > ucodeRev {
				ucodeRev = header.Rev
			}
		}

		if ucodeRev == 0 {
			s.Fatalf("No matching ucode for id %s pf %#02x", id, pf)
		}

		if uint64(ucodeRev) != rev {
			s.Fatalf("ucode rev mismatch: %#02x vs %#02x", rev, ucodeRev)
		}

		s.Logf("CPU %d id %s pf %#02x rev %#02x", idx, id, pf, rev)
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

		_, err = rdr.Seek(int64(hdr.Datasize), io.SeekCurrent)
		if err != nil {
			s.Fatal("Failed to read microcode data: ", err)
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

func extractFirmware(s *testing.State) map[string][]byte {
	vmlinux := unpackKernelImage(s)

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
			s.Fatal("Failed to read FW section entry: ", err)
		}

		_, err = ror.Seek(int64(fwe.Name-ros.Addr), io.SeekStart)
		if err != nil {
			s.Fatal("Failed to seek to name addr: ", err)
		}

		name, err := bufio.NewReader(ror).ReadString(0)
		if err != nil {
			s.Fatal("Failed to read FW section entry name: ", err)
		}
		name = strings.TrimRight(name, "\x00")

		_, err = ror.Seek(int64(fwe.Addr-ros.Addr), io.SeekStart)
		if err != nil {
			s.Fatal("Failed to seek to fw addr: ", err)
		}

		fw := make([]byte, fwe.Size)
		n, err := ror.Read(fw)
		if err != nil {
			s.Fatal("Failed to read FW section entry name: ", err)
		}
		if uint64(n) != fwe.Size {
			s.Fatal("Short firmware read")
		}

		s.Logf("FW %s (%d bytes)", name, len(fw))

		m[name] = fw
	}

	return m
}

func unpackKernelImage(s *testing.State) []byte {
	vmlinuz, err := ioutil.ReadFile("/boot/vmlinuz")
	if err != nil {
		s.Fatal("Failed to read vmlinuz: ", err)
	}

	// Search for gzip header.
	offset := bytes.Index(vmlinuz, []byte{0x1f, 0x8b, 0x08})
	if offset == -1 {
		s.Fatal("Failed to locate gzip header")
	}

	zr, err := gzip.NewReader(bytes.NewReader(vmlinuz[offset:]))
	if err != nil {
		s.Fatal("Failed to create gzip reader: ", err)
	}
	zr.Multistream(false)

	vmlinux, err := ioutil.ReadAll(zr)
	if err != nil {
		s.Fatal("Failed to decompress: ", err)
	}

	return vmlinux
}
