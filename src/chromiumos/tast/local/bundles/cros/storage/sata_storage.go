// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"encoding/json"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SATAStorage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies SATA Storage support",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

func SATAStorage(ctx context.Context, s *testing.State) {
	const (
		sataStorageDmsegCommand = "dmesg | grep SATA"
		ahciDmesgCommand        = "dmesg | grep AHCI"
		hdparmCommand           = "hdparm -I /dev/sda"
		lspciCommand            = "lspci"
		lshwCommand             = "lshw -json"
		scsiCommand             = "cat /proc/scsi/scsi"
	)

	sataOut, err := cmdOutput(ctx, sataStorageDmsegCommand)
	if err != nil {
		s.Fatal("Failed to get sataStorageDmsegCommand output: ", err)
	}

	dmsegSataRe := regexp.MustCompile(`ata\d{1}.*SATA.link.up.\d*\.\d+.Gbps.\(SStatus.\d+.SControl.\d+\)`)
	dmesgAhciRe := regexp.MustCompile(`ahci.*AHCI.*\d{2}.slots.\d{1}.*Gbps.*SATA.*`)
	if !(dmsegSataRe.MatchString(sataOut) && dmesgAhciRe.MatchString(sataOut)) {
		s.Fatal("Failed to verify SATA info in sataStorageDmsegCommand output:")

	}

	ahciOut, err := cmdOutput(ctx, ahciDmesgCommand)
	if err != nil {
		s.Fatal("Failed to get ahciDmesgCommand output: ", err)
	}

	if !dmesgAhciRe.MatchString(ahciOut) {
		s.Fatal("Failed to verify SATA info in ahciDmesgCommand output")
	}

	hdparmOut, err := cmdOutput(ctx, hdparmCommand)
	if err != nil {
		s.Fatal("Failed to get hdparmCommand output: ", err)
	}

	hdparmModelRe := regexp.MustCompile(`Model.Number:.*[A-Z0-9]+`)
	hdparmTransportRe := regexp.MustCompile(`Transport:.*Serial.*SATA.*`)
	hdparmSpeedRe := regexp.MustCompile(`speed.\(\d+\.\d+Gb\/s\)`)
	if !(hdparmModelRe.MatchString(hdparmOut) && hdparmTransportRe.MatchString(hdparmOut) && hdparmSpeedRe.MatchString(hdparmOut)) {
		s.Fatal("Failed to verify SATA info in hdparmCommand output")
	}

	scsiOut, err := cmdOutput(ctx, scsiCommand)
	if err != nil {
		s.Fatal("Failed to get scsiCommand output: ", err)
	}

	scsiRe := regexp.MustCompile(`Attached.devices:\nHost:.[a-z]*\d.[A-Z][a-z]*:.00.Id:.00.Lun:.00\n..Vendor:.ATA.*Model:.*[A-Z]\d..Rev:.\d{3}.\n.*Type:.*Direct-Access.*ANSI.*SCSI.revision:.\d{2}`)
	if !scsiRe.MatchString(scsiOut) {
		s.Fatal("Failed to verify SATA info in scsiCommand output")
	}

	hdparmStr := hdparmOut
	hdparmModelCompareRe := regexp.MustCompile(`Model Number:\s+([A-Z0-9]+)`)
	hdparmStrOut := hdparmModelCompareRe.FindStringSubmatch(hdparmStr)
	if len(hdparmStrOut) <= 1 {
		s.Fatal("Failed to get model number with hdparm command")
	}
	hdparmModel := hdparmStrOut[1]

	scsiStr := scsiOut
	scsiModelCompraeRe := regexp.MustCompile(`Model: ([A-Z0-9]+)`)
	scsiStrOut := scsiModelCompraeRe.FindStringSubmatch(scsiStr)
	if len(scsiStrOut) <= 1 {
		s.Fatal("Failed to get model number with scsi command")
	}
	scsiModel := scsiStrOut[1]

	if hdparmModel != scsiModel {
		s.Fatal("Failed to verify model Number")
	}

	lspciOut, err := cmdOutput(ctx, lspciCommand)
	if err != nil {
		s.Fatal("Failed to get lspciCommand output: ", err)
	}

	lspciRe := regexp.MustCompile(`PCI bridge: *Intel.Corporation.*\(rev.\d{2}\)`)
	if !lspciRe.MatchString(lspciOut) {
		s.Fatal("Failed to verify SATA info in lspciCommand output")
	}

	lshwOut, err := cmdOutput(ctx, lshwCommand)
	if err != nil {
		s.Fatal("Failed to get lshwCommand output: ", err)
	}

	const (
		sataStorageID   = "sata"
		descriptionName = "SATA controller"
		vendorName      = "Intel Corporation"
		productName     = "Intel Corporation"
	)

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(lshwOut), &info); err != nil {
		s.Fatal("Failed to parse JSON data: ", err)
	}

	lshwChildInfo := info["children"].([]interface{})
	var childInfoData []interface{}
	for _, inform := range lshwChildInfo {
		data := inform.(map[string]interface{})
		childInfoData = data["children"].([]interface{})
		if childInfoData != nil {
			break
		}
	}

	var infoData map[string]interface{}
	for _, inform := range childInfoData {
		infoData = inform.(map[string]interface{})
	}

	gotID := ""
	gotDescriptionName := ""
	gotProductName := ""
	gotVendorName := ""
	isIDFound := false
	isDescriptNameFound := false
	isVendorNameFound := false
	isProductNameFound := false

	for _, inform := range infoData {
		sataStorageData, ok := inform.([]interface{})
		if ok {
			for _, info := range sataStorageData {
				sataData := info.(map[string]interface{})
				gotID = sataData["id"].(string)
				if gotID != sataStorageID {
					isIDFound = true
				}
				gotDescriptionName = sataData["description"].(string)
				if gotDescriptionName == descriptionName {
					isDescriptNameFound = true
				}
				gotVendorName = sataData["vendor"].(string)
				if gotVendorName == vendorName {
					isVendorNameFound = true
				}
				gotProductName = sataData["product"].(string)
				if gotProductName == productName {
					isProductNameFound = true
				}
			}
		}
	}

	if !isIDFound {
		s.Fatalf("Unexpected SATA storage ID: got %q, want %q", gotID, sataStorageID)
	}
	if !isDescriptNameFound {
		s.Fatalf("Unexpected SATA storage descriptionName: got %q, want %q", gotDescriptionName, descriptionName)
	}
	if !isProductNameFound {
		s.Fatalf("Unexpected SATA storage productName: got %q, want %q", gotProductName, productName)
	}
	if !isVendorNameFound {
		s.Fatalf("Unexpected SATA storage vendorName: got %q, want %q", gotVendorName, vendorName)
	}
}

// cmdOutput exceutes SATA commands and returns output.
func cmdOutput(ctx context.Context, cmd string) (string, error) {
	out, err := testexec.CommandContext(ctx, "bash", "-c", cmd).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to exceute %q command", cmd)
	}
	return string(out), nil
}
