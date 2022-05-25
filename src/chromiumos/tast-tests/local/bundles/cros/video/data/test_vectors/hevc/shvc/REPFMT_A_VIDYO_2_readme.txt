Bitstream file name: REPFMT_A_VIDYO_2.hevc

Bitstream feature name: Rep format in VPS

Bitstream feature description:
rep_format( ) structure in VPS signals picture width & height, chroma format, bit depths and conformance crop window parameters.
The bitstream has 1 rep_format( ) structure signalled in VPS Extension and 2 Layers.
rep_format_idx_present_flag = 0 is inferred if vps_num_rep_formats_minus1 = 0.
vps_rep_format_idx[ i ] is inferred to be equal to Min( i, vps_num_rep_formats_minus1 ), where i=layer_id
- vps_num_rep_formats_minus1 = 0
- Rep Format 0 - pic_width_vps_in_luma_samples: 960
                 pic_height_vps_in_luma_samples: 544
                 chroma_and_bit_depth_vps_present_flag: 1
                 chroma_format_vps_idc: 1
                 bit_depth_vps_luma_minus8: 0
                 bit_depth_vps_chroma_minus8: 0
                 conformance_window_vps_flag: 1
                 conf_win_vps_bottom_offset: 2
- rep_format_idx_present_flag: 0
- Rep format Layer id map
    Layer 0 - Rep format 0
    Layer 1 - Rep format 0
- Conformance crop offsets
    Layer 0 -   Top: 0
                Bottom: 4 (signalled as 2)
                Left: 0
                Right: 0
    Layer 1 -   Top: 0
                Bottom: 4 (signalled as 2)
                Left: 0
                Right: 0
- Coding structure: Low Delay P
- Number of Frames - 4

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3.1

Each layer resolution:
- Layer 0 resolution - Output: 960x540
                       Coded:  960x544
- Layer 1 resolution - Output: 960x540
                       Coded:  960x544

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1527

Contact: Won Kap Jang, Vidyo Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

