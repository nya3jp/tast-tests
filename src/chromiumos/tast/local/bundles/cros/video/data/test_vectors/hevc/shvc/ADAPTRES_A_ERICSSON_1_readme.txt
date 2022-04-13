Bitstream file name: ADAPTRES_A_ERICSSON_1.hevc

Bitstream feature name: Adaptive resolution

Bitstream feature description:
The conformance bitstream tests the special case of skip pictures for adaptive resolution change. In adaptive resolution when switching up to higher layer, the
higher layer is coded as skip IRAP picture with P_SLICES using inter-layer picture as reference for this use case.
- Coding structure: Low delay, one reference picture
- Number of Frames - 30

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
            Layer: 2, PTL Idx: 2 (Scalable Main 4)

Number of layers: 3

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
- Layer 2 resolution - Output: 1920x1080
                       Coded:  1920x1080

Frame rate: 24 fps for all three layers

SHM Version: Independent SHVC encoder implementation used for encoding, SHM was not used. Correctly decodable by SHM-8.0.

Contact: Rickard Sjoberg, Ericsson (rickard.sjoberg@ericsson.com)
         Usman Hakeem, Ericsson (usman.hakeem@ericsson.com)
