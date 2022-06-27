// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testResources struct {
	username string // username for Chrome login
	password string // password to login
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIANoLicenseEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "GAIA Enroll a device without checking policies; domain with no licenses",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testResources{
					username: "policy.GAIANoLicenseEnrollment.user_name",
					password: "policy.GAIANoLicenseEnrollment.password",
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{
			"policy.GAIANoLicenseEnrollment.user_name",
			"policy.GAIANoLicenseEnrollment.password",
		},
	})
}

func GAIANoLicenseEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testResources)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)
	//sc := ss.NewSyslogServiceClient(cl.Conn)

	// Create reader for /var/log/messages.
	//reader, err := syslog.NewReader(ctx)
	//if err != nil {
	//	s.Fatal("Failed to initialize syslog reader: ", err)
	//}
	//defer reader.Close()
	syslogErr := make(chan error)
	checkChromeLog := func() {
		syslogErr <- func() error {
			if _, err := pc.NewReader(ctx, &ps.NewReaderRequest{}); err != nil {
				return errors.Wrap(err, "failed to get chrome logs")
			}
			return nil
		}()
	}
	go checkChromeLog()

	if _, err := pc.GAIAEnrollUsingChrome(ctx, &ps.GAIAEnrollUsingChromeRequest{
		Username:    username,
		Password:    password,
		DmserverURL: dmServerURL,
	}); err != nil {
		testing.ContextLog(ctx, "rubnz: ", err)
		//logContent, err := ioutil.ReadFile(syslog.ChromeLogFile)
		//if err != nil {
		//	s.Fatal("Failed toioutil: ", err)
		//	//return nil, errors.Wrap(err, "failed to read "+syslog.ChromeLogFile)

	}
	//testing.ContextLog(ctx, "ruben: ", syslogErr)
	//s.Fatal("Failed to enroll using chrome: ", err)
	if err := <-syslogErr; err != nil {
		s.Error("No license error: ", err)

	}
}
