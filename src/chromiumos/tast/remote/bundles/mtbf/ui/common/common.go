// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/cats"
	"chromiumos/tast/testing"
)

// GetCatsRunParams Get some parms for runing a case
func GetCatsRunParams(ctx context.Context, s *testing.State) (*cats.RunCaseParams, error) {
	hostName := s.DUT().GetHostname()
	if len(hostName) == 0 {
		return nil, mtbferrors.New(mtbferrors.OSVarRead, nil, "testing.State.DUT.GetHostname")
	}
	s.Logf("DUT Host Name [%s]", hostName)

	deviceID, err := s.DUT().GetARCDeviceID(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.OSVarRead, err, "testing.State.DUT.GetARCDeviceID")
	}
	s.Logf("DUT ARC Device ID [%s]", deviceID)

	varName := "cats.requestURL"
	requestURL, urlOk := s.Var(varName)
	if !urlOk {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, varName))
	}
	s.Logf("The Request URL [%s]", requestURL)

	deviceInfo, err := cats.QueryDeviceInfoByDutID(hostName, requestURL)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CatsQueryFailure, err)
	}
	if len(deviceInfo.NodeIP) == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodeIP, nil)
	}
	s.Logf("The CATS IP [%s]", deviceInfo.NodeIP)
	if deviceInfo.NodePort == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodePort, nil)
	}
	s.Logf("The CATS Port [%d]", deviceInfo.NodePort)
	if deviceInfo.NodeGrpcPort == 0 {
		return nil, mtbferrors.New(mtbferrors.CatsNoNodeGrpcPort, nil)
	}
	s.Logf("The CATS GRPC Port [%d]", deviceInfo.NodeGrpcPort)

	params := &cats.RunCaseParams{
		HostName:     hostName,
		RequestURL:   requestURL,
		DeviceID:     deviceID,
		NodeIP:       deviceInfo.NodeIP,
		NodePort:     deviceInfo.NodePort,
		NodeGRPCPort: deviceInfo.NodeGrpcPort,
	}
	return params, nil
}
