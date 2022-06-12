// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

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

// programmerEnumToProgrammer maps the enum from FWBackUpSection to a bios FlashromProgramer.
var programmerEnumToProgrammer = map[pb.Programmer]bios.FlashromProgrammer{
	pb.Programmer_BIOSProgrammer: bios.HostProgrammer,
	pb.Programmer_ECProgrammer:   bios.ECProgrammer,
}

// sectionEnumToSection maps the enum from FWBackUpSection to a bios ImageSection.
var sectionEnumToSection = map[pb.ImageSection]bios.ImageSection{
	pb.ImageSection_BOOTSTUBImageSection: bios.BOOTSTUBImageSection,
	pb.ImageSection_COREBOOTImageSection: bios.COREBOOTImageSection,
	pb.ImageSection_GBBImageSection:      bios.GBBImageSection,
	pb.ImageSection_ECRWImageSection:     bios.ECRWImageSection,
	pb.ImageSection_ECRWBImageSection:    bios.ECRWBImageSection,
	pb.ImageSection_EmptyImageSection:    bios.EmptyImageSection,
	pb.ImageSection_APROImageSection:     bios.APROImageSection,
	pb.ImageSection_FWSignAImageSection:  bios.FWSignAImageSection,
	pb.ImageSection_FWSignBImageSection:  bios.FWSignBImageSection,
}

// updateModeEnumtoMode maps the enum from FirmwareUpdateModeRequest to a bios FirmwareUpdateMode.
var updateModeEnumtoMode = map[pb.UpdateMode]bios.FirmwareUpdateMode{
	pb.UpdateMode_RecoveryMode: bios.RecoveryMode,
}

// BackupImageSection dumps the image region into temporary file locally and returns its path.
func (*BiosService) BackupImageSection(ctx context.Context, req *pb.FWBackUpSection) (*pb.FWBackUpInfo, error) {
	path, err := bios.NewImageToFile(ctx, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer])
	if err != nil {
		return nil, errors.Wrapf(err, "could not backup %s region with programmer %s", sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer])
	}
	return &pb.FWBackUpInfo{Path: path, Section: req.Section, Programmer: req.Programmer}, nil
}

// RestoreImageSection restores image region from temporary file locally and restores fw with it.
func (bs *BiosService) RestoreImageSection(ctx context.Context, req *pb.FWBackUpInfo) (*empty.Empty, error) {
	if err := bios.WriteImageFromSingleSectionFile(ctx, req.Path, sectionEnumToSection[req.Section], programmerEnumToProgrammer[req.Programmer]); err != nil {
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

// EnableAPSoftwareWriteProtect enables the AP software write protect.
func (bs *BiosService) EnableAPSoftwareWriteProtect(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := bios.EnableAPSoftwareWriteProtect(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// DisableAPSoftwareWriteProtect disables the AP software write protect.
// HW write protection needs to be disabled first.
func (*BiosService) DisableAPSoftwareWriteProtect(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := bios.DisableAPSoftwareWriteProtect(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// CorruptFWSection writes garbage over part of the specified firmware section.
func (bs *BiosService) CorruptFWSection(ctx context.Context, req *pb.CorruptSection) (*empty.Empty, error) {
	img, err := bios.NewImage(ctx, bios.ImageSection(sectionEnumToSection[req.Section]), programmerEnumToProgrammer[req.Programmer])
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	for i, v := range img.Data {
		img.Data[i] = (v + 1) & 0xff
	}
	err = img.WriteFlashrom(ctx, bios.ImageSection(sectionEnumToSection[req.Section]), programmerEnumToProgrammer[req.Programmer])
	if err != nil {
		return nil, errors.Wrap(err, "could not write firmware")
	}
	return &empty.Empty{}, nil
}

// WriteImageFromMultiSectionFile writes the provided multi section file in the specified section.
func (bs *BiosService) WriteImageFromMultiSectionFile(ctx context.Context, req *pb.FWBackUpInfo) (*empty.Empty, error) {
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
