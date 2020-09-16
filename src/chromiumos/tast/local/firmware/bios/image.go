// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bios

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	pb "chromiumos/tast/services/cros/firmware"
)

// ImageSection is the name of sections supported by this package.
type ImageSection string

const (
	// GBBImageSection is the named section for GBB.
	GBBImageSection ImageSection = "GBB"

	// gbbHeaderOffset is the location of the GBB header in GBBImageSection.
	gbbHeaderOffset uint = 12
)

var sortedGBBFlags []pb.GBBFlag

func init() {
	for _, v := range pb.GBBFlag_value {
		sortedGBBFlags = append(sortedGBBFlags, pb.GBBFlag(v))
	}
	sort.Slice(sortedGBBFlags, func(i, j int) bool {
		return sortedGBBFlags[i] < sortedGBBFlags[j]
	})
}

// SectionInfo represents the location and size of a firmware image section.
type SectionInfo struct {
	Start  uint
	Length uint
}

// Image represents the content and sections of a firmware image.
type Image struct {
	Data     []byte
	Sections map[ImageSection]SectionInfo
}

// NewImage creates an Image object representing the currently loaded BIOS image.
func NewImage(ctx context.Context) (*Image, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "creating tmpfile for image contents")
	}
	if err = testexec.CommandContext(ctx, "flashrom", "-p", "host", "-r", tmpFile.Name()).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "could not read firmware host image")
	}

	fmap, err := testexec.CommandContext(ctx, "dump_fmap", "-p", tmpFile.Name()).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not dump_fmap on firmware host image")
	}
	info, err := parseSections(string(fmap))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse dump_fmap output")
	}
	data, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware host image contents")
	}
	return &Image{data, info}, nil
}

// GetGBBFlags returns the list of cleared and list of set flags.
func (i *Image) GetGBBFlags() ([]pb.GBBFlag, []pb.GBBFlag, error) {
	var gbb uint32
	if err := i.readSectionData(GBBImageSection, gbbHeaderOffset, 4, &gbb); err != nil {
		return nil, nil, err
	}
	setFlags := calcGBBFlags(gbb)
	clearFlags := calcGBBFlags(^gbb)
	return clearFlags, setFlags, nil
}

// parseSections extracts section names and locations from dump_fmap output.
func parseSections(fmap string) (map[ImageSection]SectionInfo, error) {
	ret := make(map[ImageSection]SectionInfo)
	for _, line := range strings.Split(fmap, "\n") {
		// dump_fmap output line format: <section> <start pos> <length>
		if line == "" {
			continue
		}
		cols := strings.Split(line, " ")
		start, err := strconv.ParseUint(cols[1], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse section start %v", line)
		}
		length, err := strconv.ParseUint(cols[2], 10, 32)
		ret[ImageSection(cols[0])] = SectionInfo{uint(start), uint(length)}
	}
	return ret, nil
}

// calcGBBFlags interprets mask as a GBBFlag bit mask and returns the set flags.
func calcGBBFlags(mask uint32) []pb.GBBFlag {
	var res []pb.GBBFlag
	// use sorted flags to return a deterministic order of flags.
	for _, pos := range sortedGBBFlags {
		if mask&(0x0001<<pos) != 0 {
			res = append(res, pb.GBBFlag(pos))
		}
	}
	return res
}

// calcGBBMask returns the bit mask corresponding to the list of GBBFlags.
func calcGBBMask(flags []pb.GBBFlag) uint32 {
	var mask uint32
	for _, f := range flags {
		mask |= 0x0001 << f
	}
	return mask
}

// readSectionData returns interpreted data of a given size from raw bytes at the specified location.
func (i *Image) readSectionData(sec ImageSection, off, sz uint, out interface{}) error {
	si, ok := i.Sections[sec]
	if !ok {
		return errors.Errorf("Section %s not found", sec)
	}
	beg := si.Start + off
	end := si.Start + off + sz
	if len(i.Data) < int(end) {
		return errors.Errorf("Data length too short: %d (<=%d)", len(i.Data), end)
	}
	b := i.Data[beg:end]
	r := bytes.NewReader(b)
	return binary.Read(r, binary.LittleEndian, out)
}
