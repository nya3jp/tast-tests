// Copyright 2020 The ChromiumOS Authors
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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/common/firmware"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// ImageSection is the name of sections supported by this package.
type ImageSection string

// FlashromProgrammer is the type of programmer being passed to flashrom command line.
type FlashromProgrammer string

// FirmwareUpdateMode is the type of mode to perform firmware update.
type FirmwareUpdateMode string

const (
	// gbbHeaderOffset is the location of the GBB header in GBBImageSection.
	gbbHeaderOffset uint = 12

	// RecoveryMode is the named chromeOS Firmware Updater to perform firmware recovery mode.
	RecoveryMode FirmwareUpdateMode = "--mode=recovery"

	// HostProgrammer is the flashrom programmer type used to operate with AP firmware chip.
	HostProgrammer FlashromProgrammer = "host"
	// ECProgrammer is the flashrom programmer type used to operate with EC chip.
	ECProgrammer FlashromProgrammer = "ec"

	// EmptyImageSection is the empty string which will result in the whole AP/EC fw backup.
	EmptyImageSection ImageSection = ""
	// BOOTSTUBImageSection is the named section for the Coreboot image (more recent devices use COREBOOT).
	BOOTSTUBImageSection ImageSection = "BOOT_STUB"
	// COREBOOTImageSection is the named section for the Coreboot image.
	COREBOOTImageSection ImageSection = "COREBOOT"
	// SHAREDDATAImageSection is the SHARED_DATA section of the firmware.
	SHAREDDATAImageSection ImageSection = "SHARED_DATA"
	// FMAPImageSection is section containing the flash map layout.
	FMAPImageSection ImageSection = "FMAP"
	// SMMSTOREImageSection is section containing the SMMSTORE for x86 devices.
	SMMSTOREImageSection ImageSection = "SMMSTORE"

	// RECOVERYMRCCACHEImageSection is the named section for recovery MRC cache data
	RECOVERYMRCCACHEImageSection ImageSection = "RECOVERY_MRC_CACHE"
	// UNIFIEDMRCCACHEImageSection is the UNIFIED_MRC_CACHE section of the firmware.
	UNIFIEDMRCCACHEImageSection ImageSection = "UNIFIED_MRC_CACHE"
	// RWMRCCACHEImageSection is the RW_MRC_CACHE section of the firmware.
	RWMRCCACHEImageSection ImageSection = "RW_MRC_CACHE"
	// RWELOGImageSection is the RW_ELOG section of the firmware.
	RWELOGImageSection ImageSection = "RW_ELOG"
	// RWSHAREDImageSection is the RW_SHARED section of the firmware.
	RWSHAREDImageSection ImageSection = "RW_SHARED"
	// RWNVRAMImageSection is the RW_NVRAM section of the firmware.
	RWNVRAMImageSection ImageSection = "RW_NVRAM"

	// ECRWImageSection is the named section for EC writable data as output from dump_fmap.
	ECRWImageSection ImageSection = "EC_RW"
	// ECRWBImageSection is the named section for a secondary EC writable data for EFS.
	ECRWBImageSection ImageSection = "EC_RW_B"
	// ECROImageSection is the EC_RO section of the firmware.
	ECROImageSection ImageSection = "EC_RO"

	// APWPROImageSection is the the entire RO space of the flash chip.
	APWPROImageSection ImageSection = "WP_RO"
	// APROImageSection is the named readonly section for AP writable data as output from dump_fmap.
	APROImageSection ImageSection = "RO_SECTION"
	// GBBImageSection is the named section for GBB as output from dump_fmap.
	GBBImageSection ImageSection = "GBB"
	// FRMAINImageSection is the FR_MAIN section of the firmware.
	FRMAINImageSection ImageSection = "FR_MAIN"
	// RWLEGACYImageSection is the RW_LEGACY section of the firmware.
	RWLEGACYImageSection ImageSection = "RW_LEGACY"
	// RWMISCImageSection is the RW_MISC section of the firmware.
	RWMISCImageSection ImageSection = "RW_MISC"

	// ROFRIDImageSection is the RO_FRID section of the firmware.
	ROFRIDImageSection ImageSection = "RO_FRID"
	// ROFRIDPADImageSection is the RO_FRID_PAD section of the firmware.
	ROFRIDPADImageSection ImageSection = "RO_FRID_PAD"
	// RWFWIDImageSection is the RW_FWID section of the firmware.
	RWFWIDImageSection ImageSection = "RW_FWID"

	// FWSignAImageSection is the named section for Firmware A Sign as output from dump_fmap.
	FWSignAImageSection ImageSection = "VBLOCK_A"
	// FWBodyAImageSection is the named section for Firmware A Body as output from dump_fmap.
	FWBodyAImageSection ImageSection = "FW_MAIN_A"
	// APRWAImageSection is the named section A for AP writable data as output from dump_fmap.
	APRWAImageSection ImageSection = "RW_SECTION_A"
	// RWFWIDAImageSection is the RW_FWID_A section of the firmware.
	RWFWIDAImageSection ImageSection = "RW_FWID_A"

	// FWSignBImageSection is the named section for Firmware B Sign as output from dump_fmap.
	FWSignBImageSection ImageSection = "VBLOCK_B"
	// FWBodyBImageSection is the named section for Firmware B Body as output from dump_fmap.
	FWBodyBImageSection ImageSection = "FW_MAIN_B"
	// APRWBImageSection is the named section B for AP writable data as output from dump_fmap.
	APRWBImageSection ImageSection = "RW_SECTION_B"
	// RWFWIDBImageSection is the RW_FWID_B section of the firmware.
	RWFWIDBImageSection ImageSection = "RW_FWID_B"

	// ROVPDImageSection is the named section for readonly VPD data
	ROVPDImageSection ImageSection = "RO_VPD"
	// RWVPDImageSection is the RW_VPD section of the firmware.
	RWVPDImageSection ImageSection = "RW_VPD"

	// IntelCSERWAImageSection is the ME_RW_A section of the firmware in Intel.
	IntelCSERWAImageSection ImageSection = "ME_RW_A"
	// IntelCSERWBImageSection is the ME_RW_B section of the firmware in Intel.
	IntelCSERWBImageSection ImageSection = "ME_RW_B"

	// SIALLImageSection is the SI_ALL section of the firmware.
	SIALLImageSection ImageSection = "SI_ALL"
	// SIDESCImageSection is the SI_DESC section of the firmware.
	SIDESCImageSection ImageSection = "SI_DESC"
	// SIMEImageSection is the SI_ME section of the firmware.
	SIMEImageSection ImageSection = "SI_ME"
	// SIBIOSImageSection is the SI_BIOS section of the firmware.
	SIBIOSImageSection ImageSection = "SI_BIOS"
)

// sectionToSectionEnum is a direct inverse of sectionEnumToSection map in bios_service.
var sectionToSectionEnum = map[ImageSection]pb.ImageSection{
	EmptyImageSection:      pb.ImageSection_EmptyImageSection,
	BOOTSTUBImageSection:   pb.ImageSection_BOOTSTUBImageSection,
	COREBOOTImageSection:   pb.ImageSection_COREBOOTImageSection,
	SHAREDDATAImageSection: pb.ImageSection_SHAREDDATAImageSection,
	FMAPImageSection:       pb.ImageSection_FMAPImageSection,
	SMMSTOREImageSection:   pb.ImageSection_SMMSTOREImageSection,

	RECOVERYMRCCACHEImageSection: pb.ImageSection_RECOVERYMRCCACHEImageSection,
	UNIFIEDMRCCACHEImageSection:  pb.ImageSection_UNIFIEDMRCCACHEImageSection,
	RWMRCCACHEImageSection:       pb.ImageSection_RWMRCCACHEImageSection,
	RWELOGImageSection:           pb.ImageSection_RWELOGImageSection,
	RWSHAREDImageSection:         pb.ImageSection_RWSHAREDImageSection,
	RWNVRAMImageSection:          pb.ImageSection_RWNVRAMImageSection,

	ECRWImageSection:  pb.ImageSection_ECRWImageSection,
	ECRWBImageSection: pb.ImageSection_ECRWBImageSection,
	ECROImageSection:  pb.ImageSection_ECROImageSection,

	APWPROImageSection:   pb.ImageSection_APWPROImageSection,
	APROImageSection:     pb.ImageSection_APROImageSection,
	GBBImageSection:      pb.ImageSection_GBBImageSection,
	FRMAINImageSection:   pb.ImageSection_FRMAINImageSection,
	RWLEGACYImageSection: pb.ImageSection_RWLEGACYImageSection,
	RWMISCImageSection:   pb.ImageSection_RWMISCImageSection,

	ROFRIDImageSection:    pb.ImageSection_ROFRIDImageSection,
	ROFRIDPADImageSection: pb.ImageSection_ROFRIDPADImageSection,
	RWFWIDImageSection:    pb.ImageSection_RWFWIDImageSection,

	FWSignAImageSection: pb.ImageSection_FWSignAImageSection,
	FWBodyAImageSection: pb.ImageSection_FWBodyAImageSection,
	APRWAImageSection:   pb.ImageSection_APRWAImageSection,
	RWFWIDAImageSection: pb.ImageSection_RWFWIDAImageSection,

	FWSignBImageSection: pb.ImageSection_FWSignBImageSection,
	FWBodyBImageSection: pb.ImageSection_FWBodyBImageSection,
	APRWBImageSection:   pb.ImageSection_APRWBImageSection,
	RWFWIDBImageSection: pb.ImageSection_RWFWIDBImageSection,

	ROVPDImageSection: pb.ImageSection_ROVPDImageSection,
	RWVPDImageSection: pb.ImageSection_RWVPDImageSection,

	IntelCSERWAImageSection: pb.ImageSection_IntelCSERWAImageSection,
	IntelCSERWBImageSection: pb.ImageSection_IntelCSERWBImageSection,

	SIALLImageSection:  pb.ImageSection_SIALLImageSection,
	SIDESCImageSection: pb.ImageSection_SIDESCImageSection,
	SIMEImageSection:   pb.ImageSection_SIMEImageSection,
	SIBIOSImageSection: pb.ImageSection_SIBIOSImageSection,
}

// defaultChromeosFmapConversion converts dump_fmap names to those recognized by flashrom
var defaultChromeosFmapConversion = map[ImageSection]string{
	GBBImageSection:     "FV_GBB",
	FWSignAImageSection: "VBOOTA",
	FWSignBImageSection: "VBOOTB",
	FWBodyAImageSection: "FVMAIN",
	FWBodyBImageSection: "FVMAINB",
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
func NewImageToFile(ctx context.Context, section ImageSection, programmer FlashromProgrammer, dirpath string) (string, error) {
	fileDir := dirpath
	if dirpath == "" {
		fileDir = "/var/tmp"
	}
	tmpFile, err := ioutil.TempFile(fileDir, "")
	if err != nil {
		return "", errors.Wrap(err, "creating tmpfile for image contents")
	}

	frArgs := []string{"-p", string(programmer), "-r"}
	isOneSection := section != "" && section != EmptyImageSection
	if isOneSection {
		frArgs = append(frArgs, "-i", fmt.Sprintf("%s:%s", section, tmpFile.Name()))
	} else {
		frArgs = append(frArgs, tmpFile.Name())
	}

	if out, err := testexec.CommandContext(ctx, "flashrom", frArgs...).Output(testexec.DumpLogOnError); err != nil {
		os.Remove(tmpFile.Name())
		return "", errors.Wrapf(err, "could not read firmware host image: %v", string(out))
	}

	return tmpFile.Name(), nil
}

// WriteImageToFile writes image data to a file to use for flashrom command.
func (i *Image) WriteImageToFile(ctx context.Context, sec ImageSection, dirpath string) (string, error) {
	dataRange, ok := i.Sections[sec]
	if !ok {
		return "", errors.Errorf("section %q is not recognized", string(sec))
	}

	fileDir := dirpath
	if dirpath == "" {
		fileDir = "/var/tmp"
	}
	imgFile, err := ioutil.TempFile(fileDir, "")
	if err != nil {
		return "", errors.Wrap(err, "creating tmpfile for image contents")
	}

	dataToWrite := i.Data[dataRange.Start : dataRange.Start+dataRange.Length]

	if err := ioutil.WriteFile(imgFile.Name(), dataToWrite, 0644); err != nil {
		return "", errors.Wrap(err, "writing image contents to tmpfile")
	}

	return imgFile.Name(), nil
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
	// dirpath arg is irrelevant here since file gets deleted in the defer call.
	imgTmp, err := i.WriteImageToFile(ctx, sec, "")
	if err != nil {
		return errors.Wrap(err, "writing image contents to tmpfile")
	}
	defer os.Remove(imgTmp)

	// -N == no verify all. Verify is slow.
	if out, err := testexec.CommandContext(ctx, "flashrom", "-N", "-p", string(programmer), "-i", fmt.Sprintf("%s:%s", sec, imgTmp), "-w").Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "could not write host image, flashrom output: %s", string(out))
	}

	return nil
}

// WriteImageFromSingleSectionFile writes the provided single section file in the specified section.
func WriteImageFromSingleSectionFile(ctx context.Context, path string, sec ImageSection, programmer FlashromProgrammer) error {
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

// WriteImageFromMultiSectionFile writes the provided multi section file in the specified section.
func WriteImageFromMultiSectionFile(ctx context.Context, path string, sec ImageSection, programmer FlashromProgrammer) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.Wrap(err, "file does not exist")
	} else if err != nil {
		return errors.Wrap(err, "reading image from file")
	}

	// In case EmptyImageSection, no '-i' argument would be needed and the whole AP/EC will be targeted.
	frArgs := []string{"-N", "-p", string(programmer)}
	switch sec {
	case EmptyImageSection:
		frArgs = append(frArgs, "-w", path)
	default:
		// This specific syntax is required to flash a single section from a file with multiple sections on it.
		frArgs = append(frArgs, "-i", string(sec), "-w", path)
	}

	if out, err := testexec.CommandContext(ctx, "flashrom", frArgs...).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "could not write image: %v", string(out))
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

// WPArgs struct holds the optional arguments to SetAPSoftwareWriteProtect.
type WPArgs struct {
	WPRangeStart  int64
	WPRangeLength int64
	WPSection     ImageSection
}

// SetAPSoftwareWriteProtect sets write protect using flashrom.
func SetAPSoftwareWriteProtect(ctx context.Context, enable bool, args *WPArgs) error {
	// If disabling, set range to start=0, len=0. Otherwise use args to determine enable range.
	rangeStr := "--wp-range=0,0"
	enableStr := "--wp-disable"
	expState := "disabled"
	if enable {
		enableStr = "--wp-enable"
		expState = "enabled"
	}

	testing.ContextLogf(ctx, "Running flashrom with %s flag", enableStr)
	wpCmd := []string{"-p", "host", enableStr}

	if args != nil && args.WPRangeStart != -1 && args.WPRangeLength != -1 {
		rangeStr = fmt.Sprintf("--wp-range=%x,%x", args.WPRangeStart, args.WPRangeLength)
		testing.ContextLogf(ctx, "Attempting to set ap write protect on range %s", rangeStr)
		wpCmd = append(wpCmd, rangeStr)
	} else if args != nil && args.WPSection != EmptyImageSection {
		// TODO(b/247055486): There is an ongoing issue with --wp-region argument resulting
		// in segfaults and other errors, refer to bug for more details.
		tmpFile, err := ioutil.TempFile("/var/tmp", "")
		if err != nil {
			return errors.Wrap(err, "creating tmpfile to enable AP write protect")
		}
		defer os.Remove(tmpFile.Name())

		regionName := string(args.WPSection)
		regionStr := fmt.Sprintf("%s:%s", regionName, tmpFile.Name())

		// Check AP firmware WP range.
		if err := testexec.CommandContext(ctx, "flashrom", "-p", "host", "-r", "-i", regionStr).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to read the file")
		}

		wpCmd = append(wpCmd, "-i", regionStr, fmt.Sprintf("--wp-region=%s", regionName))
	} else if enable {
		// If enabling write protect with with no range or region, enable for largest available region.
		maxRange, err := findMaxAPWPRange(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get a range to attempt to write protect")
		}
		wpCmd = append(wpCmd, maxRange)
	}

	// wpCmd = append(wpCmd, rangeStr)
	if out, err := testexec.CommandContext(ctx, "flashrom", wpCmd...).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to set write protection range with flashrom: %s", string(out))
	}

	// Verify new wp status is as expected.
	if out, err := testexec.CommandContext(ctx, "flashrom", "-p", "host", "--wp-status").Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable verify write protection status with flashrom: %s", string(out))
	} else if ok := strings.Contains(string(out), fmt.Sprintf("WP: write protect is %s.", expState)); !ok {
		return errors.Errorf("expected wp status to be %q, but output was: %s", expState, string(out))
	}
	return nil
}

func findMaxAPWPRange(ctx context.Context) (string, error) {
	rangeStr := ""
	// --wp-list prints out a lot of possible ranges ordered in increasing size.
	// We expect the full range to be the last item, labelled all but for some devices this is not available.
	for i := 0; i < 3; i++ {
		out, err := testexec.CommandContext(ctx, "flashrom", "-p", "host", "--wp-list").CombinedOutput(testexec.DumpLogOnError)
		if err != nil {
			// CombinedOutput should include messages from stderr.
			if strings.Contains(string(out), "could not determine what protection ranges") {
				// Flashrom is unable to read --wp-list for ARM devices and raises this error.
				// In this case, skip retrying and jump using FMAP values.
				break
			}
			// IF failed to read --wp-list output for some other reason, then try again.
			continue
		}

		// Match output for equal sign separated range. Output looks like: `start=0x00000000 length=0x01000000 (all)`.
		// These ranges output in sorted order.
		eqlSepRange := `start=(0[xX][0-9a-fA-F]+)\s*length=(0[xX][0-9a-fA-F]+)\s*\(([^\r\n]+)\)`
		// Match output for colon separated range. Output looks like: `start: 0x000000, length: 0x1000000`.
		// These ranges are unsorted, sort by size to find max.
		colonSepRange := `start:\s*0[xX]([0-9a-fA-F]+),\s*length:\s*0[xX]([0-9a-fA-F]+)`
		match := regexp.MustCompile(eqlSepRange).FindAllStringSubmatch(string(out), -1)
		if match != nil {
			// Look for the the "all" in `start=0x00000000 length=0x01000000 (all)`.
			if match[len(match)-1][3] == "all" {
				// If the "all" range is read, return immediately.
				maxMatch := match[len(match)-1]
				rangeStr = fmt.Sprintf("--wp-range=%s,%s", maxMatch[1], maxMatch[2])
				return rangeStr, nil
			}
			// If "all" range isn't found, save second largest available but try again just to be sure.
			maxMatch := match[len(match)-1]
			rangeStr = fmt.Sprintf("--wp-range=%s,%s", maxMatch[1], maxMatch[2])
		} else if match = regexp.MustCompile(colonSepRange).FindAllStringSubmatch(string(out), -1); match != nil {
			sort.Slice(match, func(i, j int) bool {
				start1, _ := strconv.ParseInt(match[i][1], 16, 32)
				len1, _ := strconv.ParseInt(match[i][2], 16, 32)

				start2, _ := strconv.ParseInt(match[j][1], 16, 32)
				len2, _ := strconv.ParseInt(match[j][2], 16, 32)

				return (len1 - start1) < (len2 - start2) // Sort in order of increasing size.
			})
			maxMatch := match[len(match)-1]
			rangeStr = fmt.Sprintf("--wp-range=0x%s,0x%s", maxMatch[1], maxMatch[2])
			return rangeStr, nil
		}

	}
	if rangeStr != "" {
		// If any valid range was found, return it, otherwise use FMAP.
		// These ranges will definitely work for setting wp but FMAP ranges might not so it's better to use
		// best available range from --wp-list than a potentially larger range from FMAP.
		return rangeStr, nil
	}

	// If --wp-list couldn't provide a valid range/failed, use ranges from FMAP.
	tmpFile, err := ioutil.TempFile("/var/tmp", "")
	if err != nil {
		return rangeStr, errors.Wrap(err, "creating tmpfile to read FMAP into")
	}
	defer os.Remove(tmpFile.Name())

	// Check AP firmware WP range.
	if err := testexec.CommandContext(ctx, "flashrom", "-p", "host", "-r", "-i", "FMAP:"+tmpFile.Name()).Run(testexec.DumpLogOnError); err != nil {
		return rangeStr, errors.Wrap(err, "failed to read host fmap")
	}

	out, err := testexec.CommandContext(ctx, "fmap_decode", tmpFile.Name()).Output(testexec.DumpLogOnError)
	if err != nil {
		return rangeStr, errors.Wrapf(err, "failed to decode the host fmap: %v", string(out))
	}

	// Parse the output to get the areaOffset and areaSize values for write protection.
	// example output: `area_offset="0x00c00000" area_size="0x00400000" area_name="WP_RO"`
	areaRange := regexp.MustCompile(`area_offset=\"(0[xX][0-9a-fA-F]+)\" area_size=\"(0[xX][0-9a-fA-F]+)\"\s*area_name=\"WP_RO\"`)
	match := areaRange.FindStringSubmatch(string(out))
	if match == nil {
		return rangeStr, errors.Wrapf(err, "failed to parse WP_RO range in FMAP output: %v", string(out))
	}
	rangeStr = fmt.Sprintf("--wp-range=%s,%s", match[1], match[2])
	return rangeStr, nil
}

// ChromeosFirmwareUpdate will perform the firmware update in the desired mode.
func ChromeosFirmwareUpdate(ctx context.Context, mode FirmwareUpdateMode, options ...string) error {
	args := []string{string(mode)}
	if len(options) > 0 {
		args = append(args, options...)
	}
	if err := testexec.CommandContext(ctx, "chromeos-firmwareupdate", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to perform firmware update with %s", string(mode))
	}
	return nil
}

// ParseFMAP reads FMAP for given programmer then parses it into map.
func ParseFMAP(ctx context.Context, programmer FlashromProgrammer) ([]*pb.FMAP_FMAPEntry, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "creating tmpfile to read FMAP")
	}
	defer os.Remove(tmpFile.Name())

	frArgs := []string{"-p", string(programmer), "-r", "-i", fmt.Sprintf("FMAP:%s", tmpFile.Name())}
	if err = testexec.CommandContext(ctx, "flashrom", frArgs...).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "could not read FMAP with flashrom")
	}

	fmapDump, err := testexec.CommandContext(ctx, "dump_fmap", "-p", tmpFile.Name()).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not dump_fmap on firmware host image")
	}

	fmap, err := ParseSections(string(fmapDump))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse dump_fmap output")
	}

	var fmapArr []*pb.FMAP_FMAPEntry
	// Convert to output format suitable for proto message.
	for section, info := range fmap {
		fmapArr = append(fmapArr, &pb.FMAP_FMAPEntry{
			Section: sectionToSectionEnum[section],
			Range:   &pb.Range{Start: int64(info.Start), Length: int64(info.Length)},
		})
	}
	return fmapArr, nil
}
