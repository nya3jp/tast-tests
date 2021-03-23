// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bios

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"chromiumos/tast/common/firmware"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
)

// ImageSection is the name of sections supported by this package.
type ImageSection string

const (
	// GBBImageSection is the named section for GBB as output from dump_fmap.
	GBBImageSection ImageSection = "GBB"

	// gbbHeaderOffset is the location of the GBB header in GBBImageSection.
	gbbHeaderOffset uint = 12
)

// defaultChromeosFmapConversion converts dump_fmap names to those recognized by flashrom
var defaultChromeosFmapConversion = map[ImageSection]string{
	GBBImageSection: "FV_GBB",
}

// SectionInfo represents the location and size of a firmware image section.
type SectionInfo struct {
	Start  uint
	Length uint
}

// Image represents the content and sections of a firmware image.
type Image struct {
	data     []byte
	sections map[ImageSection]SectionInfo
}

// NewImage creates an Image object representing the currently loaded BIOS image.
func NewImage(ctx context.Context) (*Image, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "creating tmpfile for image contents")
	}
	defer os.Remove(tmpFile.Name())

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

// ClearAndSetGBBFlags clears and sets the specified flags, leaving the rest unchanged, set has precedence over clear.
func (i *Image) ClearAndSetGBBFlags(clearFlags, setFlags []pb.GBBFlag) error {
	var currGBB uint32
	if err := i.readSectionData(GBBImageSection, gbbHeaderOffset, 4, &currGBB); err != nil {
		return err
	}
	newGBB := calcGBBBits(currGBB, calcGBBMask(clearFlags), calcGBBMask(setFlags))
	if newGBB == currGBB {
		// No need to write section data if GBB flags are already correct.
		return nil
	}
	return i.writeSectionData(GBBImageSection, gbbHeaderOffset, newGBB)
}

// WriteFlashrom writes the current data in the specified section into flashrom.
func (i *Image) WriteFlashrom(ctx context.Context, sec ImageSection) error {
	flashromSec, ok := defaultChromeosFmapConversion[sec]
	if !ok {
		return errors.Errorf("section %q is not recognized", string(sec))
	}

	imgTmp, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "creating tmpfile for image contents")
	}
	defer os.Remove(imgTmp.Name())

	if err := ioutil.WriteFile(imgTmp.Name(), i.data, 0644); err != nil {
		return errors.Wrap(err, "writing image contents to tmpfile")
	}

	layData := i.getLayout()

	layTmp, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "creating tmpfile for layout contents")
	}
	defer os.Remove(layTmp.Name())

	if err := ioutil.WriteFile(layTmp.Name(), layData, 0644); err != nil {
		return errors.Wrap(err, "wrting layout contents to tmpfile")
	}

	if err = testexec.CommandContext(ctx, "flashrom", "-p", "host", "-l", layTmp.Name(), "-i", flashromSec, "-w", imgTmp.Name()).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "could not write host image")
	}

	return nil
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
	for _, pos := range firmware.AllGBBFlags() {
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

// calcGBBBits returns the final GBB bits after applying clear and set to curr.  Set has precedence over clear in the same bit position.
func calcGBBBits(curr, clear, set uint32) uint32 {
	return (curr & ^clear) | set
}

// readSectionData returns interpreted data of a given size from raw bytes at the specified location.
func (i *Image) readSectionData(sec ImageSection, off, sz uint, out interface{}) error {
	si, ok := i.sections[sec]
	if !ok {
		return errors.Errorf("Section %s not found", sec)
	}
	beg := si.Start + off
	end := si.Start + off + sz
	if len(i.data) < int(end) {
		return errors.Errorf("Data length too short: %d (<=%d)", len(i.data), end)
	}
	b := i.data[beg:end]
	r := bytes.NewReader(b)
	return binary.Read(r, binary.LittleEndian, out)
}

// writeSectionData writes data to the specified section location.
func (i *Image) writeSectionData(sec ImageSection, off uint, data interface{}) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, data); err != nil {
		return errors.Wrap(err, "could not parse section start")
	}

	si, ok := i.sections[sec]
	if !ok {
		return errors.Errorf("Section %s not found", sec)
	}
	bb := buf.Bytes()
	beg := si.Start + off
	if len(i.data) <= int(beg) {
		return errors.Errorf("Data length too short: %v (<=%v)", len(i.data), beg)
	}
	d := append(i.data[0:beg], bb...)
	i.data = append(d, i.data[beg+uint(len(bb)):]...)
	return nil
}

// getLayout gets the section locations of all the ones we care about into a flashrom friendly format.
func (i *Image) getLayout() []byte {
	var data []string
	for name, info := range i.sections {
		layoutName, ok := defaultChromeosFmapConversion[name]
		if !ok {
			continue
		}
		layoutStart := info.Start
		layoutEnd := layoutStart + info.Length - 1
		// lines in the layout file look like this: 0x00000001:0x0000000A FV_GBB
		data = append(data, fmt.Sprintf("0x%08x:0x%08x %s", layoutStart, layoutEnd, layoutName))
	}
	sort.Strings(data)
	return []byte(strings.Join(data, "\n") + "\n")
}
