// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/certs"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/httpserver"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// DoHMode defines possible type of DNS-over-HTTPS.
type DoHMode int

const (
	// DoHOff is a mode that resolves DNS through plaintext.
	DoHOff DoHMode = iota
	// DoHAutomatic is a mode that automatically chose between plaintext and secure DNS.
	DoHAutomatic
	// DoHAlwaysOn is a mode that resolves DNS through secure DNS.
	DoHAlwaysOn
)

// Client defines the client resolving DNS.
type Client int

const (
	// System is a DNS client type for systems.
	System Client = iota
	// User is a DNS client type for users (e.g. cups, tlsdate).
	User
	// Chrome is a DNS client type with user 'chronos'.
	Chrome
	// Crostini is a DNS client type for Crostini.
	Crostini
	// ARC is a DNS client type for ARC.
	ARC
)

// Env wraps the test environment created for DNS tests.
type Env struct {
	Router       *env.Env
	server       *env.Env
	manager      *shill.Manager
	Certs        *certs.Certs
	cleanupCerts func(context.Context)
}

// GoogleDoHProvider is the Google DNS-over-HTTPS provider.
const GoogleDoHProvider = "https://dns.google/dns-query"

// ExampleDoHProvider is a fake DNS-over-HTTPS provider used for testing using virtualnet package.
// The URL must match the CA certificate used by virtualnet/certs/cert.go.
const ExampleDoHProvider = "https://www.example.com/dns-query"

// DigProxyIPRE is the regular expressions for DNS proxy IP inside dig output.
var DigProxyIPRE = regexp.MustCompile(`SERVER: 100.115.92.\d+#53`)

// GetClientString get the string representation of a DNS client.
func GetClientString(c Client) string {
	switch c {
	case System:
		return "system"
	case User:
		return "user"
	case Chrome:
		return "Chrome"
	case Crostini:
		return "Crostini"
	case ARC:
		return "ARC"
	default:
		return ""
	}
}

// SetDoHMode updates ChromeOS setting to change DNS-over-HTTPS mode.
func SetDoHMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, mode DoHMode, dohProvider string) error {
	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPrivacy")
	if err != nil {
		return errors.Wrap(err, "failed to get connection to OS Settings")
	}
	defer conn.Close()

	ac := uiauto.New(tconn)

	// Toggle secure DNS, the UI might lag, keep trying until secure DNS is toggled to the expected state.
	leftClickAc := ac.WithInterval(2 * time.Second)
	var toggleSecureDNS = func(ctx context.Context, check checked.Checked) error {
		tb := nodewith.Role(role.ToggleButton).Name("Use secure DNS")
		var secureDNSChecked = func(ctx context.Context) error {
			tbInfo, err := ac.Info(ctx, tb)
			if err != nil {
				return errors.Wrap(err, "failed to find secure DNS toggle button")
			}
			if tbInfo.Checked != check {
				return errors.Errorf("secure DNS toggle button checked: %s", check)
			}
			return nil
		}
		if err := leftClickAc.LeftClickUntil(tb, secureDNSChecked)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle secure DNS button")
		}
		return nil
	}

	switch mode {
	case DoHOff:
		if err := toggleSecureDNS(ctx, checked.False); err != nil {
			return err
		}
		break
	case DoHAutomatic:
		if err := toggleSecureDNS(ctx, checked.True); err != nil {
			return err
		}

		rb := nodewith.Role(role.RadioButton).Name("With your current service provider")
		if err := ac.LeftClick(rb)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable automatic mode")
		}
		break
	case DoHAlwaysOn:
		if err := toggleSecureDNS(ctx, checked.True); err != nil {
			return err
		}

		// Get a handle to the input keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()

		m, err := input.Mouse(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get mouse")
		}
		defer m.Close()

		// On some devices, the text field for the provider might be hidden by the bottom bar.
		// Scroll down then focus on the text field.
		if err := m.ScrollDown(); err != nil {
			return errors.Wrap(err, "failed to scroll down")
		}

		// Find secure DNS text field through its parent.
		gcs, err := ac.NodesInfo(ctx, nodewith.Role(role.GenericContainer))
		if err != nil {
			return errors.Wrap(err, "failed to get generic container nodes")
		}
		nth := -1
		for i, e := range gcs {
			if attr, ok := e.HTMLAttributes["id"]; ok && attr == "secureDnsInput" {
				nth = i
				break
			}
		}
		if nth < 0 {
			return errors.Wrap(err, "failed to find secure DNS text field")
		}
		tf := nodewith.Role(role.TextField).Ancestor(nodewith.Role(role.GenericContainer).Nth(nth))
		if err := ac.FocusAndWait(tf)(ctx); err != nil {
			return errors.Wrap(err, "failed to focus on the text field")
		}

		rg := nodewith.Role(role.RadioGroup)
		if err := ac.WaitForLocation(rg)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for radio group")
		}
		rbsInfo, err := ac.NodesInfo(ctx, nodewith.Role(role.RadioButton).Ancestor(rg))
		if err != nil {
			return errors.Wrap(err, "failed to get secure DNS radio buttons information")
		}
		var rbLocation coords.Rect
		var found = false
		for _, e := range rbsInfo {
			if e.Name != "With your current service provider" {
				rbLocation = e.Location
				found = true
				break
			}
		}
		if !found {
			return errors.Wrap(err, "failed to find secure DNS radio button")
		}

		if err := uiauto.Combine("enable DoH always on with a custom provider",
			// Click use current service provider radio button.
			ac.MouseClickAtLocation(0, rbLocation.CenterPoint()),
			// Input a custom DoH provider.
			ac.LeftClick(tf),
			kb.AccelAction("Ctrl+A"),
			kb.AccelAction("Backspace"),
			kb.TypeAction(dohProvider),
			kb.AccelAction("Enter"),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable DoH with a custom provider")
		}
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if m, err := getDoHMode(ctx); err != nil {
			return err
		} else if m != mode {
			return errors.New("failed to get the correct DoH mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return err
	}
	return nil
}

// RandDomain returns a random domain name that can be useful for avoiding caching while testing DNS queries.
func RandDomain() string {
	return strconv.FormatInt(time.Now().UnixNano(), 16) + ".com"
}

// QueryOptions are provided to QueryDNS to configure the lookup query.
type QueryOptions struct {
	Domain     string
	Nameserver string
}

// NewQueryOptions returns a new options pre-populated with a random domain for testing.
func NewQueryOptions() *QueryOptions {
	return &QueryOptions{
		Domain: RandDomain(),
	}
}

func (o QueryOptions) digArgs() []string {
	args := []string{o.Domain}
	if o.Nameserver != "" {
		args = append(args, "@"+o.Nameserver)
	}
	return args
}

// QueryDNS resolves a domain through DNS with a specific client.
func QueryDNS(ctx context.Context, c Client, a *arc.ARC, cont *vm.Container, opts *QueryOptions) error {
	args := opts.digArgs()
	var u string
	switch c {
	case System:
		return testexec.CommandContext(ctx, "dig", args...).Run()
	case User:
		u = "cups"
	case Chrome:
		u = "chronos"
	case Crostini:
		return cont.Command(ctx, append([]string{"dig"}, args...)...).Run()
	case ARC:
		out, err := a.Command(ctx, "dumpsys", "wifi", "tools", "dns", opts.Domain).Output()
		if err != nil {
			return errors.Wrap(err, "failed to do ARC DNS query")
		}
		// At least one IP response must be observed.
		for _, l := range strings.Split(string(out), "\n") {
			if net.ParseIP(strings.TrimSpace(l)) != nil {
				return nil
			}
		}
		return errors.New("failed to resolve domain")
	default:
		return errors.New("unknown client")
	}
	return testexec.CommandContext(ctx, "sudo", append([]string{"-u", u, "dig"}, args...)...).Run()
}

// ProxyTestCase contains test case for DNS proxy tests.
type ProxyTestCase struct {
	Client     Client
	ExpectErr  bool
	AllowRetry bool
}

// TestQueryDNSProxy runs a set of test cases for DNS proxy.
func TestQueryDNSProxy(ctx context.Context, tcs []ProxyTestCase, a *arc.ARC, cont *vm.Container, opts *QueryOptions) []error {
	var errs []error
	for _, tc := range tcs {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			qErr := QueryDNS(ctx, tc.Client, a, cont, opts)
			if qErr != nil && !tc.ExpectErr {
				err = errors.Wrapf(qErr, "DNS query failed for %s", GetClientString(tc.Client))
			}
			if qErr == nil && tc.ExpectErr {
				err = errors.Errorf("successful DNS query for %s, but expected failure", GetClientString(tc.Client))
			}
			if !tc.AllowRetry {
				return testing.PollBreak(err)
			}
			return err
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// InstallDigInContainer installs dig in container.
func InstallDigInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether dig is preinstalled or not.
	if err := cont.Command(ctx, "dig", "-v").Run(); err == nil {
		return nil
	}

	// Run command sudo apt update in container. Ignore the error because this might fail for unrelated reasons.
	cont.Command(ctx, "sudo", "apt", "update").Run(testexec.DumpLogOnError)

	// Run command sudo apt install dnsutils in container.
	if err := cont.Command(ctx, "sudo", "DEBIAN_FRONTEND=noninteractive", "apt-get", "-y", "install", "dnsutils").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt install dnsutils in container")
	}

	// Run command dig -v and check the output to make sure vim has been installed successfully.
	if err := cont.Command(ctx, "dig", "-v").Run(); err != nil {
		return errors.Wrap(err, "failed to install dig in container")
	}
	return nil
}

// getDoHProviders returns the current DNS-over-HTTPS providers.
func getDoHProviders(ctx context.Context) (map[string]interface{}, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager object")
	}

	props, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	out, err := props.Get(shillconst.ManagerPropertyDOHProviders)
	if err != nil {
		return nil, err
	}
	providers, ok := out.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("property %s is not a map of string to interface: %q", shillconst.ManagerPropertyDOHProviders, out)
	}
	return providers, nil
}

// getDoHMode returns the current DNS-over-HTTPS mode.
func getDoHMode(ctx context.Context) (DoHMode, error) {
	providers, err := getDoHProviders(ctx)
	if err != nil || len(providers) == 0 {
		return DoHOff, err
	}
	for _, ns := range providers {
		if ns == "" {
			continue
		}
		return DoHAutomatic, nil
	}
	return DoHAlwaysOn, nil
}

// DigMatch runs dig to check name resolution works and verifies the expected server was used.
func DigMatch(ctx context.Context, re *regexp.Regexp, match bool) error {
	out, err := testexec.CommandContext(ctx, "dig", "google.com").Output()
	if err != nil {
		return errors.Wrap(err, "dig failed")
	}
	if re.MatchString(string(out)) != match {
		return errors.New("dig used unexpected nameserver")
	}
	return nil
}

// queryDNS queries DNS to |addr| through UDP port 53 and returns the response.
func queryDNS(ctx context.Context, msg []byte, addr string) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr+":53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write(msg); err != nil {
		return nil, err
	}

	resp := make([]byte, 512)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	return resp[:n], nil
}

// dohResponder returns a function that responds to HTTPS queries by proxying the queries to DNS server on |addr|.
func dohResponder(ctx context.Context, addr string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		msg, err := ioutil.ReadAll(req.Body)
		if err != nil {
			testing.ContextLog(ctx, "Failed to read HTTPS request: ", err)
			return
		}

		resp, err := queryDNS(ctx, msg, addr)
		if err != nil {
			testing.ContextLog(ctx, "Failed to query DNS: ", err)
			return
		}

		rw.Header().Set("content-type", "application/dns-message")
		if _, err := rw.Write(resp); err != nil {
			testing.ContextLog(ctx, "Failed to write HTTPS response: ", err)
		}
	}
}

// Cleanup cleans anything that is set up through NewEnv. This needs to be called whenever env is not needed anymore.
func (e *Env) Cleanup(ctx context.Context) {
	if e.server != nil {
		if err := e.server.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup server env: ", err)
		}
	}
	if e.Router != nil {
		if err := e.Router.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup router env: ", err)
		}
	}
	if e.cleanupCerts != nil {
		e.cleanupCerts(ctx)
		if err := restartDNSProxy(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restart DNS proxy: ", err)
		}
	}
	if e.manager != nil {
		if err := e.manager.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "ethernet,wifi,cellular"); err != nil {
			testing.ContextLog(ctx, "Failed to revert check portal list property: ", err)
		}
	}
}

// NewEnv creates a DNS environment including router that acts as the default network and a server that responds to DNS and DoH queries.
// On success, the caller is responsible to cleanup the environment through the |Cleanup| function.
func NewEnv(ctx context.Context, pool *subnet.Pool) (env *Env, err error) {
	e := &Env{}
	defer func() {
		if e != nil {
			e.Cleanup(ctx)
		}
	}()

	// Shill-related setup.
	e.manager, err = shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create manager proxy")
	}
	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := e.manager.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		return nil, errors.Wrap(err, "failed to disable portal detection on ethernet")
	}
	// Install test certificates for HTTPS server. In doing so, virtualnet/certs will mount a test certificate directory.
	// Because DNS proxy lives in its own namespace, it needs to be restarted to be able to see the test certificates.
	httpsCerts := certs.New(certs.SSLCrtPath, certificate.TestCert3())
	e.cleanupCerts, err = httpsCerts.InstallTestCerts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup certificates")
	}
	if err := restartDNSProxy(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to restart DNS proxy")
	}

	// Allocate subnet for DNS server.
	serverIPv4Subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate v4 subnet")
	}
	serverIPv6Subnet, err := pool.AllocNextIPv6Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate v6 subnet")
	}
	serverSubnetAddr := serverIPv4Subnet.IP.To4()
	// This assumes that the server will use the IPv4 address xx.xx.xx.2 from env's ConnectToRouter internal implementation.
	serverAddr := net.IPv4(serverSubnetAddr[0], serverSubnetAddr[1], serverSubnetAddr[2], 2)

	var svc *shill.Service
	svc, e.Router, err = virtualnet.CreateRouterEnv(ctx, e.manager, pool, virtualnet.EnvOptions{
		Priority:       5,
		NameSuffix:     "",
		IPv4DNSServers: []string{serverAddr.String()},
		EnableDHCP:     true,
		RAServer:       false,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up router env")
	}

	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for base service online")
	}

	e.server, err = NewServer(ctx, "server", serverIPv4Subnet, serverIPv6Subnet, e.Router, httpsCerts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up server env")
	}

	env = e
	e = nil
	return env, nil
}

// NewServer creates a server that responds to DNS and DoH queries.
func NewServer(ctx context.Context, envName string, ipv4Subnet, ipv6Subnet *net.IPNet, routerEnv *env.Env, httpsCerts *certs.Certs) (*env.Env, error) {
	success := false

	server := env.New(envName)
	if err := server.SetUp(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to set up server env")
	}
	defer func() {
		if success {
			return
		}
		if err := server.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup server env: ", err)
		}
	}()

	if err := server.ConnectToRouter(ctx, routerEnv, ipv4Subnet, ipv6Subnet); err != nil {
		return nil, errors.Wrap(err, "failed to connect server to router")
	}

	// Get server IPv4 address.
	addr, err := server.GetVethInAddrs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get env addresses")
	}

	// Start a DNS server.
	if err := server.StartServer(ctx, "dnsmasq", dnsmasq.New(dnsmasq.WithResolveHost("", addr.IPv4Addr))); err != nil {
		return nil, errors.Wrap(err, "failed to start dnsmasq")
	}

	// Start a DoH server.
	httpsserver := httpserver.New("443", dohResponder(ctx, addr.IPv4Addr.String()), httpsCerts)
	if err := server.StartServer(ctx, "httpsserver", httpsserver); err != nil {
		return nil, errors.Wrap(err, "failed to start DoH server")
	}

	success = true
	return server, nil
}
