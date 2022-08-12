// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/syslog"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterPolicyServiceServer(srv, &PolicyService{
				s:              s,
				extensionConns: make(map[string]*chrome.Conn),
			})
		},
	})
}

// PolicyService implements tast.cros.policy.PolicyService.
type PolicyService struct { // NOLINT
	s *testing.ServiceState

	chrome         *chrome.Chrome
	extensionConns map[string]*chrome.Conn
	extensionDirs  []string
	fakeDMS        *fakedms.FakeDMS
	fakeDMSDir     string
	fakeDMSRemoval bool
	chromeReader   *syslog.ChromeReader

	eds *externaldata.Server
}

func (c *PolicyService) VerifyPolicyStatus(ctx context.Context, req *ppb.VerifyPolicyStatusRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Verifying the policy is set to correct status")
	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		errors.Wrap(err, "create test API connection")
	}

	var pJSON []policy.Policy
	if err := json.Unmarshal(req.PolicyBlob, &pJSON); err != nil {
		errors.Wrap(err, "failed to unmarshal")
	}

	if err := policyutil.Verify(ctx, tconn, pJSON); err != nil {
		errors.Wrap(err, "failed to verify policies")
	}

	return &empty.Empty{}, nil
}

// StartNewChromeReader starts new syslog reader. When using this make sure to always call a function at the end
// to close the reader such as WaitForEnrollmentError.
func (c *PolicyService) StartNewChromeReader(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	chromeReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start NewReader")
	}

	c.chromeReader = chromeReader
	return &empty.Empty{}, nil
}

// WaitForEnrollmentError checks for enrollment error in logs using syslog.
func (c *PolicyService) WaitForEnrollmentError(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	const (
		logScanTimeout     = 45 * time.Second
		enrollmentErrorLog = "Enrollment error"
	)

	testing.ContextLog(ctx, "Waiting for enrollment failure message")

	for {
		entry, err := c.chromeReader.Read()
		if err == io.EOF {
			break
		}
		testing.ContextLog(ctx, entry.Content)
		if strings.Contains(entry.Content, enrollmentErrorLog) {
			testing.ContextLog(ctx, "Error message encountered")
			defer c.chromeReader.Close()
			return &empty.Empty{}, nil
		}
	}
	defer c.chromeReader.Close()
	return &empty.Empty{}, errors.New("failed to find enrollment error message")
}

// GAIAEnrollAndLoginUsingChrome enrolls the device using dmserver. Specified user is logged in after this function completes.
func (c *PolicyService) GAIAEnrollAndLoginUsingChrome(ctx context.Context, req *ppb.GAIAEnrollAndLoginUsingChromeRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with username: %s, dmserver: %s", string(req.Username), string(req.DmserverURL))

	cr, err := chrome.New(
		ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.DMSPolicy(req.DmserverURL),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr

	return &empty.Empty{}, nil
}

// GAIAEnrollUsingChrome enrolls the device using dmserver.
func (c *PolicyService) GAIAEnrollUsingChrome(ctx context.Context, req *ppb.GAIAEnrollUsingChromeRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with username: %s, dmserver: %s", string(req.Username), string(req.DmserverURL))

	cr, err := chrome.New(
		ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.NoLogin(),
		chrome.DMSPolicy(req.DmserverURL),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr

	return &empty.Empty{}, nil
}

func (c *PolicyService) GAIAEnrollForReporting(ctx context.Context, req *ppb.GAIAEnrollForReportingRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome for reporting with username: %s, dmserver: %s", string(req.Username), string(req.DmserverUrl))

	var opts []chrome.Option

	opts = append(opts, chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}))
	if req.SkipLogin {
		opts = append(opts, chrome.NoLogin())
	} else {
		opts = append(opts, chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}))
	}

	opts = append(opts, chrome.DMSPolicy(req.DmserverUrl))
	opts = append(opts, chrome.EnableFeatures(req.EnabledFeatures))
	opts = append(opts, chrome.EncryptedReportingAddr(fmt.Sprintf("%v/record", req.ReportingServerUrl)))
	opts = append(opts, chrome.ExtraArgs(req.ExtraArgs))
	opts = append(opts, chrome.CustomLoginTimeout(chrome.EnrollmentAndLoginTimeout))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr
	return &empty.Empty{}, nil
}

// EnrollUsingChrome starts a FakeDMS insstance that serves the provided policies and
// enrolls the device. Specified user is logged in after this function completes.
func (c *PolicyService) EnrollUsingChrome(ctx context.Context, req *ppb.EnrollUsingChromeRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with policy %s", string(req.PolicyJson))

	var opts []chrome.Option

	ok := false

	for _, extension := range req.Extensions {
		extDir, err := ioutil.TempDir("", "tast-extensions-")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir")
		}
		defer func(ctx context.Context) {
			if !ok {
				if err := os.RemoveAll(extDir); err != nil {
					testing.ContextLogf(ctx, "Failed to delete %s: %v", extDir, err)
				}
			}
		}(ctx)

		c.extensionDirs = append(c.extensionDirs, extDir)

		for _, file := range extension.Files {
			if err := ioutil.WriteFile(filepath.Join(extDir, file.Name), file.Contents, 0644); err != nil {
				return nil, errors.Wrapf(err, "failed to write %s for %s", file.Name, extension.Id)
			}
		}

		if extID, err := chrome.ComputeExtensionID(extDir); err != nil {
			return nil, errors.Wrap(err, "failed to compute extension id")
		} else if extID != extension.Id {
			return nil, errors.Errorf("unexpected extension id: got %s; want %s", extID, extension.Id)
		}

		opts = append(opts, chrome.UnpackedExtension(extDir))
	}

	if req.FakedmsDir == "" {
		tmpdir, err := ioutil.TempDir("", "fdms-")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir")
		}
		c.fakeDMSDir = tmpdir
		c.fakeDMSRemoval = true
	} else {
		c.fakeDMSDir = req.FakedmsDir
		c.fakeDMSRemoval = false
	}

	defer func() {
		if !ok {
			if err := os.RemoveAll(c.fakeDMSDir); err != nil {
				testing.ContextLogf(ctx, "Failed to delete %s: %v", c.fakeDMSDir, err)
			}
			c.fakeDMSDir = ""
		}
	}()

	// fakedms.New starts a background process that outlives the current context.
	fdms, err := fakedms.New(c.s.ServiceContext(), c.fakeDMSDir) // NOLINT
	if err != nil {
		return nil, errors.Wrap(err, "failed to start FakeDMS")
	}
	c.fakeDMS = fdms
	defer func() {
		if !ok {
			c.fakeDMS.Stop(ctx)
			c.fakeDMS = nil
		}
	}()

	if err := fdms.WritePolicyBlobRaw(req.PolicyJson); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	user := req.Username
	if user == "" {
		user = "tast-user@managedchrome.com"
	}

	opts = append(opts, chrome.FakeEnterpriseEnroll(chrome.Creds{User: user, Pass: "test0000"}))
	if req.SkipLogin {
		opts = append(opts, chrome.NoLogin())
	} else {
		opts = append(opts, chrome.CustomLoginTimeout(chrome.EnrollmentAndLoginTimeout))
		opts = append(opts, chrome.FakeLogin(chrome.Creds{User: user, Pass: "test0000", GAIAID: "gaiaid"}))
	}

	opts = append(opts, chrome.DMSPolicy(fdms.URL))
	opts = append(opts, chrome.ExtraArgs(req.ExtraArgs))
	opts = append(opts, chrome.EnableLoginVerboseLogs())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr

	ok = true

	return &empty.Empty{}, nil
}

// UpdatePolicies updates the policies served by FakeDMS and refreshes them in Chrome.
func (c *PolicyService) UpdatePolicies(ctx context.Context, req *ppb.UpdatePoliciesRequest) (*empty.Empty, error) {
	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

	// Write policies
	if err := c.fakeDMS.WritePolicyBlobRaw(req.PolicyJson); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Refresh policies.
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`, nil); err != nil {
		return nil, errors.Wrap(err, "failed to refresh policies")
	}

	return &empty.Empty{}, nil
}

// CheckChromeAndFakeDMS verifies that Chrome and FakeDMS are still running and responsive.
func (c *PolicyService) CheckChromeAndFakeDMS(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Check Chrome
	result := false
	if err := tconn.Eval(ctx, "true", &result); err != nil {
		return nil, errors.Wrap(err, "failed to interact with Chrome")
	}
	if !result {
		return nil, errors.New("eval 'true' returned false")
	}

	// Check FakeDMS
	if err := c.fakeDMS.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to ping FakeDMS")
	}

	return &empty.Empty{}, nil
}

// StopChromeAndFakeDMS stops Chrome and FakeDMS.
func (c *PolicyService) StopChromeAndFakeDMS(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var lastErr error

	for id, conn := range c.extensionConns {
		if err := conn.Close(); err != nil {
			testing.ContextLogf(ctx, "Failed to close connection to extension %s", id)
			lastErr = errors.Wrapf(err, "failed to close connection to extension %s", id)
		}
	}
	c.extensionConns = make(map[string]*chrome.Conn)

	if c.fakeDMS != nil {
		c.fakeDMS.Stop(ctx)
		c.fakeDMS = nil
	}

	if c.fakeDMSRemoval {
		if err := os.RemoveAll(c.fakeDMSDir); err != nil {
			testing.ContextLog(ctx, "Failed to remove temporary directory: ", err)
			lastErr = errors.Wrap(err, "failed to remove temporary directory")
		}
	}

	if c.chrome != nil {
		if err := c.chrome.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close chrome: ", err)
			lastErr = errors.Wrap(err, "failed to close chrome")
		}
		c.chrome = nil
	}

	for _, extDir := range c.extensionDirs {
		if err := os.RemoveAll(extDir); err != nil {
			testing.ContextLog(ctx, "Failed to remove extension directory: ", err)
			lastErr = errors.Wrap(err, "failed to remove extension directory")
		}
	}

	return &empty.Empty{}, lastErr
}

func (c *PolicyService) StartChrome(ctx context.Context, req *ppb.StartChromeRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Starting Chrome with policy %s", string(req.PolicyJson))

	if c.chrome != nil {
		return nil, errors.New("Chrome is already started")
	}

	var opts []chrome.Option

	if req.KeepEnrollment {
		opts = append(opts, chrome.KeepEnrollment())
	}

	user := req.Username
	if user == "" {
		user = "tast-user@managedchrome.com"
	}
	opts = append(opts, chrome.FakeLogin(chrome.Creds{User: user, Pass: "test0000", GAIAID: "gaiaid"}))

	if req.SkipLogin {
		opts = append(opts, chrome.NoLogin())
	} else if req.DeferLogin {
		opts = append(opts, chrome.DeferLogin())
	} else {
		opts = append(opts, chrome.CustomLoginTimeout(chrome.LoginTimeout))
	}

	if c.fakeDMS != nil {
		opts = append(opts, chrome.DMSPolicy(c.fakeDMS.URL))
		if err := c.fakeDMS.WritePolicyBlobRaw(req.PolicyJson); err != nil {
			return nil, errors.Wrap(err, "failed to write policy blob")
		}
	}
	opts = append(opts, chrome.EnableLoginVerboseLogs())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}

	c.chrome = cr

	return &empty.Empty{}, nil
}

func (c *PolicyService) StopChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var lastErr error

	if c.chrome == nil {
		return nil, errors.New("no active Chrome instance")
	}

	for id, conn := range c.extensionConns {
		if err := conn.Close(); err != nil {
			testing.ContextLogf(ctx, "Failed to close connection to extension %s", id)
			lastErr = errors.Wrapf(err, "failed to close connection to extension %s", id)
		}
	}
	c.extensionConns = make(map[string]*chrome.Conn)

	if err := c.chrome.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome: ", err)
		lastErr = errors.Wrap(err, "failed to close Chrome")
	}
	c.chrome = nil

	return &empty.Empty{}, lastErr
}

func (c *PolicyService) ContinueLogin(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.chrome == nil {
		return nil, errors.New("no active Chrome instance")
	}

	if err := c.chrome.ContinueLogin(ctx); err != nil {
		return nil, errors.Wrap(err, "Chrome login failed")
	}

	return &empty.Empty{}, nil
}

// CreateFakeDMSDir creates a directory. It needs to be removed with RemoveFakeDMSDir.
func (c *PolicyService) CreateFakeDMSDir(ctx context.Context, req *ppb.CreateFakeDMSDirRequest) (*empty.Empty, error) {
	// Remove existing data.
	os.RemoveAll(req.Path)

	if err := os.Mkdir(req.Path, 0755); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create FakeDMS directory")
	}

	return &empty.Empty{}, nil
}

// RemoveFakeDMSDir removes a directory created with CreateFakeDMSDir.
func (c *PolicyService) RemoveFakeDMSDir(ctx context.Context, req *ppb.RemoveFakeDMSDirRequest) (*empty.Empty, error) {
	if err := os.RemoveAll(req.Path); err != nil {
		return &empty.Empty{}, errors.Wrapf(err, "failed to remove %q", req.Path)
	}

	return &empty.Empty{}, nil
}

// StartExternalDataServer starts  an instance of externaldata.Server.
func (c *PolicyService) StartExternalDataServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.eds != nil {
		return nil, errors.New("URL Policy server already started")
	}

	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start policy HTTP server")
	}

	c.eds = eds

	return &empty.Empty{}, nil
}

// ServePolicyData serves the provided data and returns the url and hash that need to be providied to the policy.
func (c *PolicyService) ServePolicyData(ctx context.Context, req *ppb.ServePolicyDataRequest) (*ppb.ServePolicyDataResponse, error) {
	if c.eds == nil {
		return nil, errors.New("URL Policy server not started")
	}

	url, hash := c.eds.ServePolicyData(req.Contents)

	return &ppb.ServePolicyDataResponse{
		Url:  url,
		Hash: hash,
	}, nil
}

// StopExternalDataServer stops the instance of externaldata.Server.
func (c *PolicyService) StopExternalDataServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.eds == nil {
		return nil, errors.New("URL Policy server not started")
	}

	if err := c.eds.Stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop URL Policy server")
	}
	c.eds = nil

	return &empty.Empty{}, nil
}

func (c *PolicyService) connToExtension(ctx context.Context, id string) (*chrome.Conn, error) {
	if val, ok := c.extensionConns[id]; ok {
		return val, nil
	}

	bgURL := chrome.ExtensionBackgroundPageURL(id)
	conn, err := c.chrome.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to extension at %s", bgURL)
	}

	c.extensionConns[id] = conn

	return conn, nil
}

func (c *PolicyService) EvalStatementInExtension(ctx context.Context, req *ppb.EvalInExtensionRequest) (*empty.Empty, error) {
	conn, err := c.connToExtension(ctx, req.ExtensionId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection to extension")
	}

	if err := conn.Eval(ctx, req.Expression, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) EvalInExtension(ctx context.Context, req *ppb.EvalInExtensionRequest) (*ppb.EvalInExtensionResponse, error) {
	conn, err := c.connToExtension(ctx, req.ExtensionId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection to extension")
	}

	var result json.RawMessage
	if err := conn.Eval(ctx, req.Expression, &result); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode result")
	}

	return &ppb.EvalInExtensionResponse{
		Result: encoded,
	}, nil
}

func (c *PolicyService) VerifyVisibleNotification(ctx context.Context, req *ppb.VerifyVisibleNotificationRequest) (*empty.Empty, error) {
	if c.chrome == nil {
		return nil, errors.New("chrome is not available")
	}
	if req.NotificationId == "" {
		return nil, errors.New("request has empty notification id")
	}

	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	testing.ContextLogf(ctx, "Waiting for notification with id %s", req.NotificationId)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notifications, err := ash.Notifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get notifications"))
		}

		for _, notification := range notifications {
			if strings.Contains(notification.ID, req.NotificationId) {
				return nil
			}
		}
		return errors.New("failed to find notification")
	}, &testing.PollOptions{
		Timeout: 15 * time.Second, // Checks for notification once per second by default.
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for %q notification", req.NotificationId)
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) EvalExpressionInChromeURL(ctx context.Context, req *ppb.EvalExpressionInChromeUrlRequest) (*empty.Empty, error) {
	if c.chrome == nil {
		return nil, errors.New("chrome is not available")
	}
	if req.Url == "" {
		return nil, errors.New("request has empty URL")
	}
	if req.Expression == "" {
		return nil, errors.New("request has empty expression")
	}

	conn, err := c.chrome.NewConn(ctx, req.Url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", req.Url)
	}
	defer conn.Close()

	if err := conn.WaitForExprFailOnErr(ctx, req.Expression); err != nil {
		return nil, errors.Wrapf(err, "failed to evaluate expression on %s", req.Url)
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) ClientID(ctx context.Context, req *empty.Empty) (*ppb.ClientIdResponse, error) {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session_manager binding")
	}

	p, err := session.RetrievePolicyData(ctx, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve settings")
	} else if p == nil {
		return nil, errors.New("client ID not found")
	}

	return &ppb.ClientIdResponse{ClientId: *p.DeviceId}, nil
}
