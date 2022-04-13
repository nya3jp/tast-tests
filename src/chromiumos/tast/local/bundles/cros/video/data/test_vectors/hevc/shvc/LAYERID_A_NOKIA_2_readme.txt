Bitstream file name: LAYERID_A_NOKIA_2.hevc

Bitstream feature name: Gaps in layer ID

Bitstream feature description:
The bitstream has two layers with IDs 0 and 2. Layer 2 is coded as an auxiliary layer (AuxId = 2). Information related to layer 1 is not coded in VPS.

- Coding structure: Hierarchical B-frames with GOP size of 8

- Number of Frames - 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 2, PTL Idx: 2 (Main 4)
- OLS_2 -   Layer: 2, PTL Idx: 2 (Main 4)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3.1
- PTL_Idx 2 - Profile: MAIN [Auxiliary layer PTL]
              Level: 4

Each layer resolution: 1280x720

Frame rate: 24 fps

Software used:
  - HTM encoder 16.0 with modification for encoding independent non-base layer.
  - SHM-dev decoder r1527 with assert preventing main profile in layers with layer ID > 0 disabled.

Contact: Miska Hannuksela, Nokia Technologies (miska.hannuksela@nokia.com)
         Antti Hallapuro,  Nokia Technologies (antti.hallapuro@nokia.com)
