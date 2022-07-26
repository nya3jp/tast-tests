// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterBiosServiceServer(srv, &BiosService{s: s})
		},
	})
}

// BiosService implements tast.cros.firmware.BiosService.
type BiosService struct {
	s *testing.ServiceState
}

// programmerEnumToProgrammer maps the enum from FWSectionInfo to a bios FlashromProgramer.
var programmerEnumToProgrammer = map[pb.Programmer]bios.FlashromProgrammer{
	pb.Programmer_BIOSProgrammer: bios.HostProgrammer,
	pb.Programmer_ECProgrammer:   bios.ECProgrammer,
}

// sectionEnumToSection maps the enum from FWSectionInfo to a bios ImageSection.
var sectionEnumToSection = map[pb.ImageSection]bios.ImageSection{
	pb.ImageSection_BOOTSTUBImageSection:         bios.BOOTSTUBImageSection,
	pb.ImageSection_COREBOOTImageSection:         bios.COREBOOTImageSection,
	pb.ImageSection_GBBImageSection:              bios.GBBImageSection,
	pb.ImageSection_ECRWImageSection:             bios.ECRWImageSection,
	pb.ImageSection_ECRWBImageSection:            bios.ECRWBImageSection,
	pb.ImageSection_EmptyImageSection:            bios.EmptyImageSection,
	pb.ImageSection_APROImageSection:             bios.APROImageSection,
	pb.ImageSection_RECOVERYMRCCACHEImageSection: bios.RECOVERYMRCCACHEImageSection,
	pb.ImageSection_ROVPDImageSection:            bios.ROVPDImageSection,
	pb.ImageSection_RWVPDImageSection:            bios.RWVPDImageSection,
	pb.ImageSection_FWSignAImageSection:          bios.FWSignAImageSection,
	pb.ImageSection_FWSignBImageSection:          bios.FWSignBImageSection,
	pb.ImageSection_FWBodyAImageSection:          bios.FWBodyAImageSection,
	pb.ImageSection_FWBodyBImageSection:          bios.FWBodyBImageSection,
	pb.ImageSection_APRWAImageSection:            bios.APRWAImageSection,
	pb.ImageSection_APRWBImageSection:            bios.APRWBImageSection,
	pb.ImageSection_APWPROImageSection:           bios.APWPROImageSection,
}

// updateModeEnumtoMode maps the enum from FirmwareUpdateModeRequest to a bios FirmwareUpdateMode.
var updateModeEnumtoMode = map[pb.UpdateMode]bios.FirmwareUpdateMode{
	pb.UpdateMode_RecoveryMode: bios.RecoveryMode,
}

// BackupImageSection dumps the image region into temporary file locally and returns its path.
func (*BiosService) BackupImageSection(ctx context.Context, req *pb.FWSectionInfo) (*pb.FWSectionInfo, error) {
	path, err := bios.NewImageToFile(ctx, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer], req.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not backup %s region with programmer %s", sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer])
	}
	return &pb.FWSectionInfo{Path: path, Section: req.Section, Programmer: req.Programmer}, nil
}

// RestoreImageSection restores image region from temporary file locally and restores fw with it.
func (bs *BiosService) RestoreImageSection(ctx context.Context, req *pb.FWSectionInfo) (*empty.Empty, error) {
	if err := bios.WriteImageFromMultiSectionFile(ctx, req.Path, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer]); err != nil {
		return nil, errors.Wrapf(err, "could not restore %s region with programmer %s from path %s", sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer], req.Path)
	}
	return &empty.Empty{}, nil
}

// GetGBBFlags gets the flags that are cleared and set.
func (*BiosService) GetGBBFlags(ctx context.Context, req *empty.Empty) (*pb.GBBFlagsState, error) {
	img, err := bios.NewImage(ctx, bios.GBBImageSection, bios.HostProgrammer)
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	cf, sf, err := img.GetGBBFlags()
	if err != nil {
		return nil, errors.Wrap(err, "could not get GBB flags")
	}
	ret := pb.GBBFlagsState{Clear: cf, Set: sf}
	return &ret, nil
}

// ClearAndSetGBBFlags clears and sets specified GBB flags, leaving the rest unchanged.
func (bs *BiosService) ClearAndSetGBBFlags(ctx context.Context, req *pb.GBBFlagsState) (*empty.Empty, error) {
	bs.s.Logf("Start ClearAndSetGBBFlags: %v", req)
	img, err := bios.NewImage(ctx, bios.GBBImageSection, bios.HostProgrammer)
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	if err = img.ClearAndSetGBBFlags(req.Clear, req.Set); err != nil {
		return nil, errors.Wrap(err, "could not clear/set flags")
	}
	if err = img.WriteFlashrom(ctx, bios.GBBImageSection, bios.HostProgrammer); err != nil {
		return nil, errors.Wrap(err, "could not write image")
	}
	return &empty.Empty{}, nil
}

// SetAPSoftwareWriteProtect sets the AP software write protect.
func (bs *BiosService) SetAPSoftwareWriteProtect(ctx context.Context, req *pb.WPRequest) (*empty.Empty, error) {
	args := &bios.WPArgs{
		WPRangeStart:  -1, // Fill with default values initially.
		WPRangeLength: -1,
		WPSection:     bios.EmptyImageSection,
	}
	if req.Range != nil {
		args.WPRangeStart = req.Range.Start
		args.WPRangeLength = req.Range.Length
	} else if req.Section != pb.ImageSection_EmptyImageSection {
		args.WPSection = sectionEnumToSection[req.Section]
	}
	if err := bios.SetAPSoftwareWriteProtect(ctx, req.Enable, args); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// CorruptFWSection writes garbage over part of the specified firmware section.
// Provide a dir to save corrupted image in the request, else temp image file will be cleaned up.
func (bs *BiosService) CorruptFWSection(ctx context.Context, req *pb.FWSectionInfo) (*pb.FWSectionInfo, error) {
	img, err := bios.NewImage(ctx, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer])
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	for i, v := range img.Data {
		img.Data[i] = (v + 1) & 0xff
	}

	// Save copy of corrupted data to file before writing.
	corruptedImg, err := img.WriteImageToFile(ctx, sectionEnumToSection[req.Section], req.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed writing image contents to file")
	}
	// Delete temporary image file if saving not requested with req.Path.
	if req.Path == "" {
		defer os.Remove(corruptedImg)
	}

	// Write corrupted image with flashrom.
	err = bios.WriteImageFromMultiSectionFile(ctx, corruptedImg, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer])
	if err != nil {
		return nil, errors.Wrap(err, "could not write firmware")
	}

	// Return path to corrupted fw file if save path provided.
	if req.Path == "" {
		return &pb.FWSectionInfo{Section: req.Section, Programmer: req.Programmer}, nil
	}
	return &pb.FWSectionInfo{Path: corruptedImg, Section: req.Section, Programmer: req.Programmer}, nil
}

// WriteImageFromMultiSectionFile writes the provided multi section file in the specified section.
func (bs *BiosService) WriteImageFromMultiSectionFile(ctx context.Context, req *pb.FWSectionInfo) (*empty.Empty, error) {
	if err := bios.WriteImageFromMultiSectionFile(ctx, req.Path, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer]); err != nil {
		return nil, errors.Wrapf(err, "could not write %s region with programmer %s from path %s", sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer], req.Path)
	}
	return &empty.Empty{}, nil
}

// ChromeosFirmwareUpdate will perform the firmware update in the desired mode.
func (*BiosService) ChromeosFirmwareUpdate(ctx context.Context, req *pb.FirmwareUpdateModeRequest) (*empty.Empty, error) {
	switch req.Options {
	case "":
		if err := bios.ChromeosFirmwareUpdate(ctx, updateModeEnumtoMode[req.Mode]); err != nil {
			return nil, err
		}
	default:
		if err := bios.ChromeosFirmwareUpdate(ctx, updateModeEnumtoMode[req.Mode], req.Options); err != nil {
			return nil, err
		}
	}
	return &empty.Empty{}, nil
}
