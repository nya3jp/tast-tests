// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"regexp"
	"strings"

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
		lshwCommand             = "lshw"
		scsiCommand             = "cat /proc/scsi/scsi"
	)

	var (
		dmsegSataRe          = regexp.MustCompile(`ata\d{1}.*SATA.link.up.\d*\.\d+.Gbps.\(SStatus.\d+.SControl.\d+\)`)
		dmesgAhciRe          = regexp.MustCompile(`ahci.*AHCI.*\d{2}.slots.\d{1}.*Gbps.*SATA.*`)
		hdparmModelRe        = regexp.MustCompile(`Model.Number:.*[A-Z0-9]+`)
		hdparmTransportRe    = regexp.MustCompile(`Transport:.*Serial.*SATA.*`)
		hdparmSpeedRe        = regexp.MustCompile(`speed.\(\d+\.\d+Gb\/s\)`)
		scsiRe               = regexp.MustCompile(`Attached.devices:\nHost:.[a-z]*\d.[A-Z][a-z]*:.00.Id:.00.Lun:.00\n..Vendor:.ATA.*Model:.*[A-Z]\d..Rev:.\d{3}.\n.*Type:.*Direct-Access.*ANSI.*SCSI.revision:.\d{2}`)
		hdparmModelCompareRe = regexp.MustCompile(`Model Number:\s+([A-Z0-9]+)`)
		scsiModelCompraeRe   = regexp.MustCompile(`Model: ([A-Z0-9]+)`)
		deviceKeyRe          = regexp.MustCompile(`^\s*\*-([a-z:0-9]+|[a-z:0-9]+.*)$`)
		lspciRe              = regexp.MustCompile(`PCI bridge: *Intel.Corporation.*\(rev.\d{2}\)`)
	)

	sataOut, err := cmdOutput(ctx, sataStorageDmsegCommand)
	if err != nil {
		s.Fatal("Failed to get sataStorageDmsegCommand output: ", err)
	}

	if !(dmsegSataRe.MatchString(sataOut) && dmesgAhciRe.MatchString(sataOut)) {
		s.Fatal("Failed to verify SATA info in sataStorageDmsegCommand output:")

	}

	ahciOut, err := cmdOutput(ctx, ahciDmesgCommand)
	if err != nil {
		s.Fatal("Failed to get ahciDmesgCommand output: ", err)
	}

	if !dmesgAhciRe.MatchString(ahciOut) {
		s.Fatal("Failed to verify SATA info in ahciDmesgCommand output:")
	}

	hdparmOut, err := cmdOutput(ctx, hdparmCommand)
	if err != nil {
		s.Fatal("Failed to get hdparmCommand output: ", err)
	}

	if !(hdparmModelRe.MatchString(hdparmOut) && hdparmTransportRe.MatchString(hdparmOut) && hdparmSpeedRe.MatchString(hdparmOut)) {
		s.Fatal("Failed to verify SATA info in hdparmCommand output")
	}

	scsiOut, err := cmdOutput(ctx, scsiCommand)
	if err != nil {
		s.Fatal("Failed to get scsiCommand output: ", err)
	}

	if !scsiRe.MatchString(scsiOut) {
		s.Fatal("Failed to verify SATA info in scsiCommand output")
	}

	hdparmStr := hdparmOut
	hdparmStrOut := hdparmModelCompareRe.FindStringSubmatch(hdparmStr)
	hdparmModel := hdparmStrOut[1]

	scsiStr := scsiOut
	scsiStrOut := scsiModelCompraeRe.FindStringSubmatch(scsiStr)
	scsiModel := scsiStrOut[1]

	if hdparmModel != scsiModel {
		s.Fatal("Failed to verify model Number")
	}

	lspciOut, err := cmdOutput(ctx, lspciCommand)
	if err != nil {
		s.Fatal("Failed to get lspciCommand output: ", err)
	}

	if !lspciRe.MatchString(lspciOut) {
		s.Fatal("Failed to verify SATA info in lspciCommand output")
	}

	lshwOut, err := cmdOutput(ctx, lshwCommand)
	if err != nil {
		s.Fatal("Failed to get lshwCommand output: ", err)
	}

	deviceKey := "sata"
	var message []string
	flag := false
	for _, line := range strings.Split(lshwOut, "\n") {
		key := deviceKeyRe.FindStringSubmatch(string(line))
		if len(key) != 0 && deviceKey == key[1] {
			flag = true
			continue
		}
		if !flag {
			continue
		}
		if flag && deviceKeyRe.MatchString(string(line)) {
			break
		}
		keyPair := strings.SplitN(line, ":", 2)
		if len(keyPair) == 2 {
			message = append(message, strings.TrimSpace(keyPair[1]))
		}
	}

	if !flag {
		s.Fatal("Failed to verify lshw command Output for SATA information")
	}

	const (
		descriptionName = "SATA controller"
		productName     = "Intel Corporation"
		vendorName      = "Intel Corporation"
	)

	if len(message) > 1 {
		if got := message[0]; got != descriptionName {
			s.Fatalf("Failed to get SATA storage description info: got %q; want %q", got, descriptionName)
		}
		if got := message[1]; got != productName {
			s.Fatalf("Failed to get SATA storage product info: got %q; want %q", got, productName)
		}
		if got := message[2]; got != vendorName {
			s.Fatalf("Failed to get SATA storage vendor info: got %q; want %q", got, vendorName)
		}
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
