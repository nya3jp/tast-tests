// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cats

import (
	"chromiumos/tast/common/mtbferrors"
)

const errWithUsrErrCode = "5300"
const usrErrWithReturn = "9999"

// RunCaseParams contains the parameters for c-ats case run.
type RunCaseParams struct {
	HostName        string
	RequestURL      string
	CaseName        string
	CaseDesc        string
	TaskName        string
	DeviceID        string
	GrpcReportPath  string
	GrpcDroneLogDir string
	NodeIP          string
	NodePort        int
	NodeGRPCPort    int
	query           bool
}

func (p *RunCaseParams) validate() *mtbferrors.MTBFError {
	if len(p.CaseName) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoCaseName, nil)
	}
	if len(p.CaseName) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoDUTName, nil)
	}
	if len(p.RequestURL) != 0 {
		p.query = true
		return nil
	}
	if len(p.DeviceID) == 0 || len(p.NodeIP) == 0 {
		return mtbferrors.New(mtbferrors.CatsNoQueryURL, nil)
	}
	p.query = false
	return nil
}

//DeepCopy copies a RunCaseParams
func (p *RunCaseParams) DeepCopy() *RunCaseParams {
	return &RunCaseParams{
		HostName:     p.HostName,
		RequestURL:   p.RequestURL,
		CaseName:     p.CaseName,
		TaskName:     p.TaskName,
		DeviceID:     p.DeviceID,
		NodeIP:       p.NodeIP,
		NodePort:     p.NodePort,
		NodeGRPCPort: p.NodeGRPCPort,
		query:        p.query,
	}
}
