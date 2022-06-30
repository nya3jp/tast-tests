## Data archives for cross_version_login tests

These data archives contain snapshots of user data vaults and the TPM simulator
states, collected on various versions of Chrome OS.

### Collecting snapshots

To collect such a snapshot from a Chrome OS version 91.* or newer, use the
`hwsec-test-utils/cross_version_login/prepare_cross_version_login_data.sh`
script.

To collect snapshots for older versions, manual steps are required, as the test
would require building a custom OS image with a few tpm2-simulator, cryptohome
and trunks patches backported.
