Bitstream file name: REFREGOFF_A_SHARP_1.hevc

Bitstream feature name: Reference region offsets in PPS

Bitstream feature description:
The reference region offset parameters in the PPS (starting with ref_region_offset_present_flag) are used to indicate the region on the reference picture which corresponds to the scaled reference layer region defined by scaled reference layer offset parameters.

- Reference layer offsets
    Layer 0 -   (NOT SIGNALLED)
    Layer 1 -   Top: 100 (signalled as 50)
                Bottom: 180 (signalled as 90)
                Left: 540 (signalled as 270)
                Right: 220 (signalled as 110)
    Note: 1280x800 EL picture corresponds to 640x400 region of BL picture, thus ratio is 2x.
- Coding structure: Random Access
- Number of Frames - 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 4.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 4.1)
	        Layer: 1, PTL Idx: 2 (Scalable Main 4.1)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 2
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4.1
- PTL_Idx 1 - Profile: SCALABLE MAIN [Ehhancement layer PTL]
              Level: 4.1

Each layer resolution:
- Layer 0 resolution - Output: 1280x800
                       Coded:  1280x800
- Layer 1 resolution - Output: 1280x800
                       Coded:  1280x800

Frame rate: 30 fps for both layers

SHM Version: SHM-dev rev1033

Contact: Tomoyuki Yamamoto, SHARP (yamamoto.tomoyuki@sharp.co.jp)

