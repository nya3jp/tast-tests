package network

import (
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"strings"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WlanDriver,
		Desc: "Ensure wireless devices have the expected associated kernel driver.",
		Contacts: []string{
			// FIXME: check it's the right person
			"kirtika@chromium.org", // Connectivity team
			"oka@chromium.org",     // Tast port author
		},
	})
}

func WlanDriver(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		name          string
		versionToPath map[string]string
	}{
		{
			name: "Atheros AR9280",
			versionToPath: map[string]string{
				"3.4":  "wireless/ath/ath9k/ath9k.ko",
				"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
				"4.14": "wireless/ath/ath9k/ath9k.ko",
				"4.19": "wireless/ath/ath9k/ath9k.ko",
			},
		},
	} {
		cmd := testexec.CommandContext(ctx, "uname", "-r")
		kernelRelease, err := cmd.Output()
		if err != nil {
			s.Fatal("Failed to get kernel release: ", err)
		}
		baseRevision = strings.Join(strings.Spt(kernelRelease, ".")[:2], ".")

		m, err := shill.NewManager(ctx)
		if err != nil {
			s.Fatal("Failed to get shill manager: ", err)
		}

		uninit, err := m.FindHoge()
		if err != nil {
			s.Fatal("Failed to get uninitialized technologies: ", err)
		}
		m.FindProperty()

		prop = "UninitializedTechnologies"

		deviceInfoRoot = "sys/class/net"

	}
	//           "Atheros AR9382": {
	//                   "3.4": "wireless/ath/ath9k/ath9k.ko",
	//                   "3.8": "wireless-3.4/ath/ath9k/ath9k.ko",
	//                   "4.14": "wireless/ath/ath9k/ath9k.ko",
	//                   "4.19": "wireless/ath/ath9k/ath9k.ko",
	//           },
	//           "Intel 7260": {
	//                   "3.8": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.4": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//           },
	//           "Intel 7265": {
	//                   "3.8": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "3.18": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.4": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//           },
	//           "Intel 9000": {
	//                   "4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//           },
	//           "Intel 9260": {
	//                   "4.4": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//           },
	//           "Intel 22260": {
	//                   "4.4": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//                   "4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	//           },
	//           "Atheros AR9462": {
	//                   "3.4": "wireless/ath/ath9k_btcoex/ath9k_btcoex.ko",
	//                   "3.8": "wireless-3.4/ath/ath9k_btcoex/ath9k_btcoex.ko",
	//                   "4.14": "wireless/ath/ath9k/ath9k.ko",
	//                   "4.19": "wireless/ath/ath9k/ath9k.ko",
	//           },
	//           "Qualcomm Atheros QCA6174": {
	//                   "4.4": "wireless/ar10k/ath/ath10k/ath10k_pci.ko",
	//                   "4.14": "wireless/ath/ath10k/ath10k_pci.ko",
	//                   "4.19": "wireless/ath/ath10k/ath10k_pci.ko",
	//           },
	//           "Qualcomm Atheros QCA6174 SDIO": {
	//                   "4.19": "wireless/ath/ath10k/ath10k_sdio.ko",
	//           },
	//           "Qualcomm WCN3990": {
	//                   "4.14": "wireless/ath/ath10k/ath10k_snoc.ko",
	//                   "4.19": "wireless/ath/ath10k/ath10k_snoc.ko",
	//           },
	//           "Marvell 88W8797 SDIO": {
	//                   "3.4": "wireless/mwifiex/mwifiex_sdio.ko",
	//                   "3.8": "wireless-3.4/mwifiex/mwifiex_sdio.ko",
	//                   "4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//                   "4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//           },
	//           "Marvell 88W8887 SDIO": {
	//                    "3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
	//                    "4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//                    "4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//           },
	//           "Marvell 88W8897 PCIE": {
	//                    "3.8": "wireless/mwifiex/mwifiex_pcie.ko",
	//                    "3.10": "wireless-3.8/mwifiex/mwifiex_pcie.ko",
	//                    "4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	//                    "4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	//           },
	//           "Marvell 88W8897 SDIO": {
	//                    "3.8": "wireless/mwifiex/mwifiex_sdio.ko",
	//                    "3.10": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
	//                    "3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
	//                    "3.18": "wireless/mwifiex/mwifiex_sdio.ko",
	//                    "4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//                    "4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	//           },
	//           "Broadcom BCM4354 SDIO": {
	//                    "3.8": "wireless/brcm80211/brcmfmac/brcmfmac.ko",
	//                    "3.14": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
	//                    "4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	//                    "4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	//           },
	//           "Broadcom BCM4356 PCIE": {
	//                    "3.10": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
	//                    "4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	//                    "4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	//           },
	//           "Marvell 88W8997 PCIE": {
	//                    "4.4": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	//                    "4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	//                    "4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	//           },

}
