// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bios

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/common/firmware"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
)

// ImageSection is the name of sections supported by this package.
type ImageSection string

// FlashromProgrammer is the type of programmer being passed to flashrom command line.
type FlashromProgrammer string

const (
	// HostProgrammer is the flashrom programmer type used to operate with AP firmware chip.
	HostProgrammer FlashromProgrammer = "host"

	// ECProgrammer is the flashrom programmer type used to operate with EC chip.
	ECProgrammer FlashromProgrammer = "ec"

	// BOOTSTUBImageSection is the named section for the Coreboot image (more recent devices use COREBOOT).
	BOOTSTUBImageSection ImageSection = "BOOT_STUB"

	// COREBOOTImageSection is the named section for the Coreboot image.
	COREBOOTImageSection ImageSection = "COREBOOT"

	// GBBImageSection is the named section for GBB as output from dump_fmap.
	GBBImageSection ImageSection = "GBB"

	// ECRWImageSection is the named section for EC writable data as output from dump_fmap.
	ECRWImageSection ImageSection = "EC_RW"

	// ECRWBImageSection is the named section for a secondary EC writable data for EFS.
	ECRWBImageSection ImageSection = "EC_RW_B"

	// EmptyImageSection is the empty string which will result in the whole AP/EC fw backup.
	EmptyImageSection ImageSection = ""

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
	Data     []byte
	Sections map[ImageSection]SectionInfo
}

// NewImageFromData creates an Image object from an in memory image.
func NewImageFromData(data []byte, sections map[ImageSection]SectionInfo) *Image {
	return &Image{data, sections}
}

// NewImage creates an Image object representing the currently loaded BIOS image. If you pass in a section, only that section will be read.
func NewImage(ctx context.Context, section ImageSection, programmer FlashromProgrammer) (*Image, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "creating tmpfile for image contents")
	}
	defer os.Remove(tmpFile.Name())

	frArgs := []string{"-p", string(programmer), "-r"}
	isOneSection := section != ""
	if isOneSection {
		frArgs = append(frArgs, "-i", fmt.Sprintf("%s:%s", section, tmpFile.Name()))
	} else {
		frArgs = append(frArgs, tmpFile.Name())
	}

	if err = testexec.CommandContext(ctx, "flashrom", frArgs...).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "could not read firmware host image")
	}

	data, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware host image contents")
	}
	var info map[ImageSection]SectionInfo
	if !isOneSection {
		fmap, err := testexec.CommandContext(ctx, "dump_fmap", "-p", tmpFile.Name()).Output(testexec.DumpLogOnError)
		if err != nil {
			return nil, errors.Wrap(err, "could not dump_fmap on firmware host image")
		}
		info, err = ParseSections(string(fmap))
		if err != nil {
			return nil, errors.Wrap(err, "could not parse dump_fmap output")
		}
	} else {
		info = make(map[ImageSection]SectionInfo)
		info[section] = SectionInfo{
			Start:  0,
			Length: uint(len(data)),
		}
	}
	return &Image{data, info}, nil
}

// NewImageToFile creates a file representing the desired section of currently loaded firmware image.
func NewImageToFile(ctx context.Context, section ImageSection, programmer FlashromProgrammer) (string, error) {
	tmpFile, err := ioutil.TempFile("/var/tmp", "")
	if err != nil {
		return "", errors.Wrap(err, "creating tmpfile for image contents")
	}

	frArgs := []string{"-p", string(programmer), "-r"}
	isOneSection := section != ""
	if isOneSection {
		frArgs = append(frArgs, "-i", fmt.Sprintf("%s:%s", section, tmpFile.Name()))
	} else {
		frArgs = append(frArgs, tmpFile.Name())
	}

	if err = testexec.CommandContext(ctx, "flashrom", frArgs...).Run(testexec.DumpLogOnError); err != nil {
		os.Remove(tmpFile.Name())
		return "", errors.Wrap(err, "could not read firmware host image")
	}

	return tmpFile.Name(), nil
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
func (i *Image) WriteFlashrom(ctx context.Context, sec ImageSection, programmer FlashromProgrammer) error {
	dataRange, ok := i.Sections[sec]
	if !ok {
		return errors.Errorf("section %q is not recognized", string(sec))
	}

	imgTmp, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "creating tmpfile for image contents")
	}
	defer os.Remove(imgTmp.Name())

	dataToWrite := i.Data[dataRange.Start : dataRange.Start+dataRange.Length]

	if err := ioutil.WriteFile(imgTmp.Name(), dataToWrite, 0644); err != nil {
		return errors.Wrap(err, "writing image contents to tmpfile")
	}

	// -N == no verify all. Verify is slow.
	if err = testexec.CommandContext(ctx, "flashrom", "-N", "-p", string(programmer), "-i", fmt.Sprintf("%s:%s", sec, imgTmp.Name()), "-w").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "could not write host image")
	}

	return nil
}

// WriteImageFromFile writes the provided path in the specified section of the firmware
func WriteImageFromFile(ctx context.Context, path string, sec ImageSection, programmer FlashromProgrammer) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.Wrap(err, "file does not exist")
	} else if err != nil {
		return errors.Wrap(err, "reading image from file")
	}

	if err := testexec.CommandContext(ctx, "flashrom", "-N", "-p", string(programmer), "-i", fmt.Sprintf("%s:%s", sec, path), "-w").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "could not write host image")
	}

	return nil
}

// ParseSections extracts section names and locations from dump_fmap output.
func ParseSections(fmap string) (map[ImageSection]SectionInfo, error) {
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

// writeSectionData writes data to the specified section location.
func (i *Image) writeSectionData(sec ImageSection, off uint, data interface{}) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, data); err != nil {
		return errors.Wrap(err, "could not parse section start")
	}

	si, ok := i.Sections[sec]
	if !ok {
		return errors.Errorf("Section %s not found", sec)
	}
	bb := buf.Bytes()
	beg := si.Start + off
	if len(i.Data) <= int(beg) {
		return errors.Errorf("Data length too short: %v (<=%v)", len(i.Data), beg)
	}
	d := append(i.Data[0:beg], bb...)
	i.Data = append(d, i.Data[beg+uint(len(bb)):]...)
	return nil
}

// GetLayout gets the section locations of all the ones we care about into a flashrom friendly format.
func (i *Image) GetLayout() []byte {
	var data []string
	for name, info := range i.Sections {
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

// EnableAPSoftwareWriteProtect enables and specifies the RO region for the AP.
func EnableAPSoftwareWriteProtect(ctx context.Context) error {
	tmpFile, err := ioutil.TempFile("/var/tmp", "")
	if err != nil {
		return errors.Wrap(err, "creating tmpfile to enable AP write protect")
	}
	defer os.Remove(tmpFile.Name())

	// Check AP firmware WP range.
	if err := testexec.CommandContext(ctx, "flashrom", "-p", "host", "-r", "-i", "FMAP:"+tmpFile.Name()).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to read the bios file")
	}

	out, err := testexec.CommandContext(ctx, "fmap_decode", tmpFile.Name()).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to decode the bios file")
	}

	// Parse the output to get the areaOffset and areaSize values for write protection.
	stringv := strings.Split(string(out), "\n")
	var areaOffset string
	var areaSize string
	for _, line := range stringv {
		if strings.Contains(line, "WP_RO") {
			values := strings.Split(line, "\"")
			areaOffset = values[1]
			areaSize = values[3]
			break
		}
	}

	// Declare the starting and ending range to run in the flashrom command for write protection.
	command := fmt.Sprintf("%v,%v", areaOffset, areaSize)
	if err = testexec.CommandContext(ctx, "flashrom", "-p", "host", "--wp-enable", "--wp-range", command).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to run the declared write protection range in flashrom")
	}
	return nil
}
