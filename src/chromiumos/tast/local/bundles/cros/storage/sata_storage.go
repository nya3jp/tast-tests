// Copyright 2022 The ChromiumOS Authors
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

type lshwInfo struct {
	// ID contains expected children device hardware ID
	// like network, memory, sata, etc.
	ID string `json:"id"`
	// Class contains expected children device category class
	// like bus, network, storage, etc.
	Class string `json:"class"`
	// Claimed contains claim status of hardware device
	// like true or false.
	Claimed bool `json:"claimed"`
	// Handle contains handler of hardware device
	// like PCI:0000:00:14.2, USB:4, etc.
	Handle string `json:"handle"`
	// Description contains hardware device interface description
	// like  Bluetooth wireless interface, SATA controller, etc.
	Description string `json:"description"`
	// Product contains hardware device product name
	// like Intel Corporation, Tiger Lake-LP Shared SRAM, etc.
	Product string `json:"product"`
	// Vendor contains hardware device vendor name
	// like Intel Corporation, Realtek Semiconductor Co., Ltd, etc.
	Vendor string `json:"vendor"`
	// Children contains array of Children.
	Children []*lshwInfo `json:"children"`
}

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
		pciID           = "pci"
		sataStorageID   = "sata"
		sataDescription = "SATA controller"
		sataVendor      = "Intel Corporation"
		sataProduct     = "Intel Corporation"
	)

	var result lshwInfo
	if err := json.Unmarshal([]byte(lshwOut), &result); err != nil {
		s.Fatal("Failed to parse JSON data: ", err)
	}
	sataFound := false

	for _, node := range result.Children {
		if node.Children != nil {
			for _, node1 := range node.Children {
				if node1.ID == pciID {
					for _, node2 := range node1.Children {
						if node2.ID == sataStorageID {
							sataFound = true
							if node2.Description != sataDescription {
								s.Fatalf("Unexpected SATA storage description: got %q, want %q", node2.Description, sataDescription)
							}
							if node2.Vendor != sataVendor {
								s.Fatalf("Unexpected SATA storage vendor: got %q, want %q", node2.Vendor, sataVendor)
							}
							if node2.Product != sataProduct {
								s.Fatalf("Unexpected SATA storage product: got %q, want %q", node2.Product, sataProduct)
							}
						}
					}
				}
			}
		}
	}

	if !sataFound {
		s.Fatal("Failed to find sata storage ID in lshw output")
	}
}

// cmdOutput executes SATA commands and returns output.
func cmdOutput(ctx context.Context, cmd string) (string, error) {
	out, err := testexec.CommandContext(ctx, "bash", "-c", cmd).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to exceute %q command", cmd)
	}
	return string(out), nil
}
