Bitstream file name: RESCHANGE_B_VIDYO_1.bit

Bitstream feature name: Resolution change in SPS

Bitstream feature description:
rep_format( ) structure in VPS signals picture width & height, chroma format, bit depths and conformance crop window parameters.
For base layer (layer_id = 0) or independent non-base layer, SPS signals picture width & height, chroma format, bit depths and conformance crop window parameters.
These parameters can override values signalled by VPS rep_format( ).
The bitstream has 2 rep_format( ) structures signalled in VPS Extension. SPS for layer 0 signals new resolution.
- vps_num_rep_formats_minus1 = 1
- Rep Format 0 - pic_width_vps_in_luma_samples: 1280
                 pic_height_vps_in_luma_samples: 720
                 chroma_and_bit_depth_vps_present_flag: 1
                 chroma_format_vps_idc: 1
                 bit_depth_vps_luma_minus8: 0
                 bit_depth_vps_chroma_minus8: 0
                 conformance_window_vps_flag: 0
- Rep Format 1 - pic_width_vps_in_luma_samples: 1920
                 pic_height_vps_in_luma_samples: 1080
                 chroma_and_bit_depth_vps_present_flag: 1
                 chroma_format_vps_idc: 1
                 bit_depth_vps_luma_minus8: 0
                 bit_depth_vps_chroma_minus8: 0
                 conformance_window_vps_flag: 0
- rep_format_idx_present_flag: 1
- vps_rep_format_idx[ 0 ] = 0 (default value not signalled)
  vps_rep_format_idx[ 1 ] = 1
- In SPS for Layer 0 - chroma_format_idc: 1
                       pic_width_in_luma_samples: 960
                       pic_height_in_luma_samples: 536
                       conformance_window_flag: 0
                       bit_depth_luma_minus8: 0
                       bit_depth_chroma_minus8: 0
- In SPS for Layer 1 - update_rep_format_flag: 0
- Coding structure: Low Delay P
- Number of Frames - 4

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
- Layer 0 resolution - Coded:  960x536
- Layer 1 resolution - Coded:  1920x1080

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1007

Contact: Jill Boyce, Vidyo Inc. (jill@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

