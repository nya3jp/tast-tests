// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterCgptServiceServer(srv, &CgptService{s: s})
		},
	})
}

// CgptService implements tast.cros.firmware.CgptService.
type CgptService struct {
	s *testing.ServiceState
}

// GetCgptTable returns structure containing metadata with CGPT partitions
func (c *CgptService) GetCgptTable(ctx context.Context, req *pb.GetCgptTableRequest) (resp *pb.GetCgptTableResponse, err error) {
	c.s.Logf("Reading CGPT table for device %s...", req.BlockDevice)
	cgptOut, err := testexec.CommandContext(ctx, "cgpt", "show", string(req.BlockDevice)).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve cgpt table")
	}

	cgptOutLines := strings.Split(string(cgptOut), "\n")
	partition_table := []*pb.CgptPartition{}

	for idx, line := range cgptOutLines {
		if strings.Contains(line, "Label:") {
			fields := strings.Fields(line)

			partition_start, err := strconv.Atoi(fields[0])
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse partition start offset")
			}

			partition_size, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse partition size")
			}

			partition_number, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse partition numbers")
			}

			partition_path := fmt.Sprintf("%sp%d", string(req.BlockDevice), partition_number)
			partition_label := strings.ReplaceAll(fields[4], "\"", "")

			partition_type := ""
			partition_uuid := ""
			partition_attrs := []*pb.CgptPartitionAttribute{}

			if strings.Contains(cgptOutLines[idx+1], "Type:") {
				partitionTypeFields := strings.Fields(cgptOutLines[idx+1])
				partition_type = strings.Join(partitionTypeFields[1:], " ")
			}

			if strings.Contains(cgptOutLines[idx+2], "UUID:") {
				partitionUUIDFields := strings.Fields(cgptOutLines[idx+2])
				partition_uuid = partitionUUIDFields[1]
			}

			if strings.Contains(cgptOutLines[idx+3], "Attr:") {
				partitionAttrFields := strings.Fields(cgptOutLines[idx+3])
				for _, field := range partitionAttrFields[1:] {
					attrFields := strings.Split(field, "=")

					attr_name := attrFields[0]
					attr_value, err := strconv.Atoi(attrFields[1])
					if err != nil {
						return nil, errors.Wrap(err, "failed to parse partition attribute value")
					}

					partition_attribute := &pb.CgptPartitionAttribute{
						Name:  attr_name,
						Value: int32(attr_value),
					}
					partition_attrs = append(partition_attrs, partition_attribute)
				}
			}

			partition := &pb.CgptPartition{
				PartitionPath:   partition_path,
				PartitionNumber: int32(partition_number),
				Start:           int32(partition_start),
				Size:            int32(partition_size),
				Label:           partition_label,
				Type:            partition_type,
				UUID:            partition_uuid,
				Attrs:           partition_attrs,
			}

			partition_table = append(partition_table, partition)
		}
	}

	return &pb.GetCgptTableResponse{
		CgptTable: partition_table,
	}, nil
}

// GetRawHeader returns the raw header of CGPT partition (first 4096 bytes)
func (c *CgptService) GetRawHeader(ctx context.Context, req *pb.GetRawHeaderRequest) (*pb.GetRawHeaderResponse, error) {
	if err := testexec.CommandContext(ctx, "dd", "if="+req.PartitionPath, "of=/tmp/cgpt-header", "bs=4096", "count=1", "conv=sync").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to read raw header from partition")
	}
	rawHeader, err := os.ReadFile("/tmp/cgpt-header")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read raw header dump")
	}
	return &pb.GetRawHeaderResponse{
		RawHeader: rawHeader,
	}, nil
}

// WriteRawHeader writes the raw CGPT header into chosen partitionpartition
func (c *CgptService) WriteRawHeader(ctx context.Context, req *pb.WriteRawHeaderRequest) (*empty.Empty, error) {
	if err := os.WriteFile("/tmp/cgpt-header", req.RawHeader, os.FileMode(0666)); err != nil {
		return &emptypb.Empty{}, errors.Wrap(err, "failed to save raw header into temporary file")
	}
	if err := testexec.CommandContext(ctx, "dd", "if=/tmp/cgpt-header", "of="+req.PartitionPath, "bs=4096", "count=1", "conv=sync").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to write raw header into partition")
	}
	return &emptypb.Empty{}, nil
}

// RestoreCgptAttributes restores CGPT partition attributes directly dumped from GetCgptTable
func (c *CgptService) RestoreCgptAttributes(ctx context.Context, req *pb.RestoreCgptAttributesRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Restoring passed CGPT attributes to: ", req.BlockDevice)
	for _, part := range req.CgptTable {
		if len(part.Attrs) > 0 {
			cgptAddCmdline := []string{"add", "-i", strconv.Itoa(int(part.PartitionNumber))}
			for _, attr := range part.Attrs {
				switch attr.Name {
				case "legacy_boot":
					cgptAddCmdline = append(cgptAddCmdline, "-B", strconv.Itoa(int(attr.Value)))
				case "priority":
					cgptAddCmdline = append(cgptAddCmdline, "-P", strconv.Itoa(int(attr.Value)))
				case "tries":
					cgptAddCmdline = append(cgptAddCmdline, "-T", strconv.Itoa(int(attr.Value)))
				case "successful":
					cgptAddCmdline = append(cgptAddCmdline, "-S", strconv.Itoa(int(attr.Value)))
				case "required":
					cgptAddCmdline = append(cgptAddCmdline, "-R", strconv.Itoa(int(attr.Value)))
				}
			}
			cgptAddCmdline = append(cgptAddCmdline, req.BlockDevice)
			c.s.Log("Restoring CGPT metadata: ", strings.Join(cgptAddCmdline, " "))
			if err := testexec.CommandContext(ctx, "cgpt", cgptAddCmdline...).Run(testexec.DumpLogOnError); err != nil {
				return &emptypb.Empty{}, errors.Wrap(err, "failed to restore cgpt attributes")
			}
		}
	}
	return &emptypb.Empty{}, nil
}
