Bitstream file name: PSEXT_A_VIDYO_2.hevc

Bitstream feature name: SPS, PPS additional extension

Bitstream feature description:
sps_extension_5bits (Rec. ITU-T H.265 v3 (04/2015)) is reserved for future use by ITU-T | ISO/IEC.
When present, sps_extension_5bits shall be equal to 0 in bitstreams conforming to version 2 of the Specification. However, decoders shall allow the value of sps_extension_6bits to be not equal to 0 and shall ignore all sps_extension_data_flag syntax elements in an SPS NAL unit.
The same applies to pps_extension_5bits & pps_extension_data_flag syntax elements.
This bitstream has these syntax elements set as below.
- Base layer PPS - pps_extension_5bits: 0x9
- Enhancement layer PPS - pps_extension_5bits: 0x16
- Base layer SPS - sps_extension_5bits: 0
- Enhancement layer SPS - sps_extension_5bits: 0x19
- Coding structure: Low Delay P
- Number of Frames - 4

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3.1
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3.1

Each layer resolution:
- Layer 0 resolution - Coded:  1280x720
- Layer 1 resolution - Coded:  1280x720

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1527

Contact: Won Kap Jang, Vidyo Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

