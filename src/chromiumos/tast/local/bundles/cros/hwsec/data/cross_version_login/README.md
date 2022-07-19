## Data archives for cross_version_login tests

These data archives contain snapshots of user data vaults and the TPM simulator
states, collected on various versions of ChromeOS.

### Collecting snapshots

To collect such a snapshot, use the
`hwsec-test-utils/cross_version_login/prepare_cross_version_login_data.py`
script.

Note that some snapshots are collected from "custombuild" VMs, which are
manually built versions of ChromeOS with multiple patches backported in order to
enable full testing (e.g., M90 and earlier lacked fully functioning TPM
simulator). These VM images, as well as instructions on how they were created,
are stored at
[private Google Cloud Storage folders](https://pantheon.corp.google.com/storage/browser/chromeos-test-assets-private/tast/cros/hwsec/cross_version_login/custombuilds).
