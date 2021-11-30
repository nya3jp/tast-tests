// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package intelfwextractor extracts the fw dump and validate its contents.
package intelfwextractor

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	yoyoMagic    = 0x14789633 // HrP2, CcP2, JfP2, ThP2
	validMagic   = 0x14789632 // StP2
	dumpInfoType = 1 << 31
	csrFile      = "csr"
	monitorFile  = "monitor"
	lmacDccmFile = "dccm_lmac"
)

// DevcoreHeader is a struct that contains the fw dump header information.
type DevcoreHeader struct {
	Magic    uint32
	FileSize uint32
}

// TLHeader is a struct that contains the type and length of a TLV header.
type TLHeader struct {
	Type   uint32
	Length uint32
}

// VHeader is a struct that contains the memory type and address of a value in a  TLV header.
type VHeader struct {
	MemType uint32
	Addr    uint32
}

// VYoYoMemHeader is a struct for reading the value of a YoYo TLV header.
type VYoYoMemHeader struct {
	Version     uint32
	RegionID    uint32
	NumOfRanges uint32
	NameLength  uint32
}

// VYoYoInfoHeader is a struct for reading the value of a YoYo TLV header.
type VYoYoInfoHeader struct {
	Version          uint32
	TriggerTimepoint uint32
	TriggerReason    uint32
	ExternalCfgState uint32
	VerType          uint32
	VerSubtype       uint32
	HwStep           uint32
	HwType           uint32
	RfIDFlavor       uint32
	RfIDDash         uint32
	RfIDStep         uint32
	RfIDType         uint32
	LmacMajor        uint32
	LmacMinor        uint32
	UmacMajor        uint32
	UmacMinor        uint32
	FwMonMode        uint32
	RegionsMask      uint64
}

// MandatoryFWDumpFiles are the mandaotry files that must exists in the fw dump
// to be valid. Refer to http://b/169152720#comment28 for more information.
// These files are only mandatory for the non YOYO_Magic firmware dumps.
// csr.lst       (mandatory) header type 1
// monitor.lst   (mandatory) header type 5
// dccm_lmac.lst (mandatory) header type of 9
type MandatoryFWDumpFiles struct {
	Csr      bool
	Monitor  bool
	DccmLmac bool
}

// ValidateFWDump extracts and validate the contents of the fw dump.
func ValidateFWDump(ctx context.Context, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return errors.Wrapf(err, "failed to open the file %s", file)
	}
	defer f.Close()

	var r io.ReadCloser
	r, err = gzip.NewReader(f)
	if err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}
	defer r.Close()

	fwDumpData, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrap(err, "failed to read the fw_dump")
	}

	fwDumpBuffer := bytes.NewBuffer(fwDumpData)

	// Validate the header.
	fwDumpHeader := DevcoreHeader{}
	err = binary.Read(fwDumpBuffer, binary.LittleEndian, &fwDumpHeader)
	if err != nil {
		return errors.Wrap(err, "failed to read the header of the fw dump")
	}

	// Check the fw dump size.
	if len(fwDumpData) != int(fwDumpHeader.FileSize) {
		return errors.Errorf("unexpected file size: got %d, want %d", len(fwDumpData), fwDumpHeader.FileSize)
	}

	// Check the magic signature of the fw dump.
	if fwDumpHeader.Magic == yoyoMagic {
		testing.ContextLog(ctx, "FW magic Signature: YOYO_Magic")
		if err := validateYoYoMagicFWDump(fwDumpBuffer); err != nil {
			return err
		}
	} else if fwDumpHeader.Magic == validMagic {
		testing.ContextLog(ctx, "FW magic Signature: not YOYO_Magic")
		var mFiles MandatoryFWDumpFiles
		if err := validateNonYoYoMagicFWDump(fwDumpBuffer, &mFiles); err != nil {
			return err
		}
	} else {
		return errors.Errorf("invalid magic signature: got %x, want  {%x, %x}", fwDumpHeader.Magic, validMagic, yoyoMagic)
	}

	return nil
}

func validateYoYoMagicFWDump(fwDumpBuffer *bytes.Buffer) error {
	// Parse the fw dump.
	tlHeader := TLHeader{}
	var yoyoExpectedFileRegionIDs []int
	var yoyoFoundFileRegionIDs []int

	for fwDumpBuffer.Len() > 0 {
		// Read a TLV header.
		err := binary.Read(fwDumpBuffer, binary.LittleEndian, &tlHeader)
		if err != nil {
			return errors.Wrap(err, "failed to read the header of the fw dump")
		}

		// Continue if the TLV header type is 0.
		if tlHeader.Type == 0 {
			fwDumpBuffer.Next(4)
			continue
		}

		if tlHeader.Length > 0 {
			// Read the TLV header value.
			if tlHeader.Type == dumpInfoType {
				// If the header is dumpInfoType then it should have the Region IDs for the
				// files the should be in the fw dump. For more info refer to http://b/169152720#comment78
				// The expected region IDs are saved in the slice yoyoExpectedFileRegionIDs that is used
				// to check if all expected files exists in the fw dump.

				// Extract the value from the buffer using the tlHeader.Length.
				value := *fwDumpBuffer
				value.Truncate(int(tlHeader.Length))
				vYoYoInfoHeader := VYoYoInfoHeader{}
				// Read the VYoYoInfoHeader to get the file name length.
				err = binary.Read(&value, binary.LittleEndian, &vYoYoInfoHeader)
				if err != nil {
					return errors.Wrap(err, "failed to read the header of the fw dump")
				}

				i := 0
				for i < 64 {
					if ((1 << i) & vYoYoInfoHeader.RegionsMask) > 0 {
						yoyoExpectedFileRegionIDs = append(yoyoExpectedFileRegionIDs, i)
					}
					i++
				}
			} else {
				// Extract the value from the buffer using the tlHeader.Length.
				value := *fwDumpBuffer
				value.Truncate(int(tlHeader.Length))
				vYoYoMemHeader := VYoYoMemHeader{}
				// Read the VYoYoMemHeader to get the file name length.
				err = binary.Read(&value, binary.LittleEndian, &vYoYoMemHeader)
				if err != nil {
					return errors.Wrap(err, "failed to read the header of the fw dump")
				}

				yoyoFoundFileRegionIDs = append(yoyoFoundFileRegionIDs, int(vYoYoMemHeader.RegionID))
			}
		}
		fwDumpBuffer.Next(int(tlHeader.Length))
	}

	// Check that all expected files exists in the fw dump.
	for e := range yoyoExpectedFileRegionIDs {
		found := false
		for f := range yoyoFoundFileRegionIDs {
			if e == f {
				found = true
			}
		}
		if !found {
			return errors.Errorf("failed due to at least one of the expected files is missing: got %v, want %v", yoyoFoundFileRegionIDs, yoyoExpectedFileRegionIDs)
		}
	}

	return nil
}

func validateNonYoYoMagicFWDump(fwDumpBuffer *bytes.Buffer, mandatoryFiles *MandatoryFWDumpFiles) error {
	// Check the type of the memory API.
	newMemAPI, err := checkMemoryAPI(fwDumpBuffer)
	if err != nil {
		return err
	}

	tlHeader := TLHeader{}
	vHeader := VHeader{}

	sramFiles := false
	wifiDeviceName := ""

	// Parse the fw dump.
	for fwDumpBuffer.Len() > 0 {
		// Read the TLV header.
		err := binary.Read(fwDumpBuffer, binary.LittleEndian, &tlHeader)
		if err != nil {
			return errors.Wrap(err, "failed to read the header of the fw dump")
		}

		// Continue if the TLV header type is 0.
		if tlHeader.Type == 0 {
			fwDumpBuffer.Next(4)
			continue
		}

		if tlHeader.Length == 0 {
			continue
		}

		// Check the type of the TLV header.
		switch tlHeader.Type {
		case 1:
			mandatoryFiles.Csr = true
		case 4:
			// Read device name.
			// Extract the TLV value from the buffer using the tlHeader.Length.
			hValue := *fwDumpBuffer
			hValue.Next(72)
			hValue.Truncate(64)
			wifiDeviceName = hValue.String()
		case 5:
			mandatoryFiles.Monitor = true
		case 9:
			// Extract the TLV value from the buffer using the tlHeader.Length.
			hValue := *fwDumpBuffer
			hValue.Truncate(int(tlHeader.Length))

			// Read Memory Type and Address from the TLV value using vHeader.
			err = binary.Read(&hValue, binary.LittleEndian, &vHeader)
			if err != nil {
				return errors.Wrap(err, "failed to read the header of the fw dump")
			}
			if newMemAPI == 0 {
				if vHeader.MemType == 0 {
					sramFiles = true
				}
			} else {
				// Check the memory type of the header.
				if vHeader.MemType == 0 {
					mandatoryFiles.DccmLmac = true
				}
			}
		}

		fwDumpBuffer.Next(int(tlHeader.Length))
	}

	if newMemAPI == 0 {
		// Check the name of the WiFi chip.
		if strings.Contains(wifiDeviceName, "7260") || strings.Contains(wifiDeviceName, "7265") || strings.Contains(wifiDeviceName, "a620") {
			// Check if sram files has been created.
			if sramFiles {
				mandatoryFiles.DccmLmac = true
			}
		} else if strings.Contains(wifiDeviceName, "8000") || strings.Contains(wifiDeviceName, "8260") {
			mandatoryFiles.DccmLmac = true
		}
	}

	// Check if the mandatory files exists in the fw dump.
	if !mandatoryFiles.Csr || !mandatoryFiles.Monitor || !mandatoryFiles.DccmLmac {
		return errors.Errorf("One of the mandatory files id messing: got {csr: %t, monitor: %t, dccm_lmac: %t}, want {csr: True, monitor: True, dccm_lmac: True}", mandatoryFiles.Csr, mandatoryFiles.Monitor, mandatoryFiles.DccmLmac)
	}

	return nil
}

func checkMemoryAPI(fwDumpBuffer *bytes.Buffer) (int, error) {
	checkMemAPIData := *fwDumpBuffer
	newMemAPI := 0
	tlHeader := TLHeader{}
	vHeader := VHeader{}
	for checkMemAPIData.Len() > 0 {
		// Read the TLV header.
		err := binary.Read(&checkMemAPIData, binary.LittleEndian, &tlHeader)
		if err != nil {
			return 0, errors.Wrap(err, "failed to read the header of the fw dump")
		}

		// Check the TLV header type.
		if tlHeader.Type == 0 {
			checkMemAPIData.Next(4)
			continue
		}

		if tlHeader.Type == 9 {
			// Extract the header value from the buffer using the tlHeader.Length.
			value := checkMemAPIData
			value.Truncate(int(tlHeader.Length))

			// Read Memory Type and Address from the TLV value using vHeader.
			err = binary.Read(&value, binary.LittleEndian, &vHeader)
			if err != nil {
				return 0, errors.Wrap(err, "failed to read the header of the fw dump")
			}

			// Check if the memory type is 2.
			if vHeader.MemType == 2 {
				newMemAPI = 1
			}
		}
		checkMemAPIData.Next(int(tlHeader.Length))
	}
	return newMemAPI, nil
}
