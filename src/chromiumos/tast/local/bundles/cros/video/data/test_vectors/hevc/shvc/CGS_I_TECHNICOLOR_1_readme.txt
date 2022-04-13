Bitstream file name: CGS_I_TECHNICOLOR_1.hevc

Bitstream feature name: Spatial ratio change with Colour Gamut Scalability

Bitstream feature description:
The bitstream has two layers. The base layer video format is 8-bit 960x540 BT.709 and the enhancement layer video format is 10-bit 1920x1080 BT.2020
cm_octant_depth = 1 | split_octant_flag = 0

Number of Frames: 2

Number of layers: 2

Each layer resolution:
- Layer 0 resolution : 960x540 8b    - Color Space : BT. 709
- Layer 1 resolution : 1920X1080 10b - Color Space : BT. 2020

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 4.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 4.1)
            Layer: 1, PTL Idx: 2 (Scalable Main10 4.1)

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 4.1
- PTL_Idx 2 - Profile: SCALABLE MAIN10 [Enhancement layer PTL]
              Level: 4.1

Frame rate: 60 fps for both layers

SHM Version: SHM-8.0 tag (rev 1026)

Important note: the SHM-8.0 does not decode correctly pictures with different bit-depth in BL and EL (see ticket #65)
- to decode the Layer-0 reconstructed pictures, use "-olsidx 0" option
- to decode the Layer-1 reconstructed pictures, use "-olsidx 1" option, but the base layer pictures are wrong.

Contact: Franck Hiron, technicolor (franck.hiron@technicolor.com)