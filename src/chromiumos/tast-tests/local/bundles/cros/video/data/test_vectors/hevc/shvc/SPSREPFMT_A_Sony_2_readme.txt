SPSREPFMT_A_Sony

Specification: All slices are coded as I, P or B slices. Each picture contains one slice.
vps_max_layers_minus1 is equal to 1. vps_num_rep_formats_minus1 is equal to 3.

Functional stage: Test representation format update in SPS

Frame rate: 24 fps

Description:
The bitstream has two layers (See LX below for the coding width and height).
Four sets of representation format are signalled in VPS (See rep_formatX below for the signalled width and height).
In VPS, the default inference rule of the representation formant is applied to the layers.
In L1 SPS, the representation format is updated.

Number of layers: 2

L0: 960x540
L1: 960x540

rep_format0: 960x540
rep_format1: 1280x720
rep_format2: 1920x1080
rep_format3: 3840x2160

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 5.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 5.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 5.1)

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 5.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 5.1
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 5.1

