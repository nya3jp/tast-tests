Bitstream file name: POC_B_Ericsson_1.hevc

Bitstream feature name: Unaligned POC

Bitstream feature description:
The conformance bitstream tests poc_reset_idc = 1 with vps_poc_lsb_aligned_flag = 0. poc_reset_idc equal to 1 specifies that only the most significant bits of the picture order count value for the current picture are reset.
- Coding structure: Low delay, one reference picture
- Number of Frames - 10

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3.1
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 1280x720
                       Coded:  1280x720
- Layer 1 resolution - Output: 1920x1080
                       Coded:  1920x1080

Frame rate: 24 fps for all layers

SHM Version: Independent SHVC encoder implementation used for encoding, SHM was not used. Correctly decodable by SHM-dev (rev. 1513).

Contact: Rickard Sjoberg, Ericsson (rickard.sjoberg@ericsson.com)
         Usman Hakeem, Ericsson (usman.hakeem@ericsson.com)
