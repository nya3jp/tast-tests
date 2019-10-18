///////////////////////////////////////////////////////////////////////////////
// 163. DeviceLocalAccounts
// This policy can be modified without rebooting.
///////////////////////////////////////////////////////////////////////////////
type DeviceLocalAccounts struct {
    Stat Status
    Val  []*Localaccounts
}

type Localaccounts struct {
    AppType       int             `json:"type"`
    AccountID     string          `json:"account_id"`
    Kioskapp     *LocalaccountsKioskapp  `json:"kiosk_app"`
}

type LocalaccountsKioskapp struct {
    AppID         string          `json:"app_id"`
}

func (p *DeviceLocalAccounts) Name() string          { return "DeviceLocalAccounts" }
func (p *DeviceLocalAccounts) Field() string         { return "device_local_accounts.account" }
func (p *DeviceLocalAccounts) Scope() Scope          { return DeviceScope }
func (p *DeviceLocalAccounts) Status() Status        { return p.Stat }
func (p *DeviceLocalAccounts) UntypedV() interface{} { return p.Val }
func (p *DeviceLocalAccounts) UnmarshalAs(m json.RawMessage) (interface{}, error) {
    var v []*Localaccounts
    if err := json.Unmarshal(m, &v); err != nil {
        return nil, errors.Wrapf(err, "could not read %s as Localaccounts", m)
    }
    return v, nil
}
func (p *DeviceLocalAccounts) Equal(iface interface{}) bool {
    v, ok := iface.([]*Localaccounts)
    if !ok {
        return ok
    }
    return cmp.Equal(p.Val, v)
}
