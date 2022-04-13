Bitstream file name: SLLEV_A_VIDYO_1.hevc

Bitstream feature name: Sub-layer level signalling

Bitstream feature description:
In the Profile, Tier & Level syntax structure, sub_layer_level_present_flag[i] (i varies from 0 to maxNumSubLayers-2) specifies if the level information is present in the profile_tier_level( ) syntax structure for the sub-layer representation with TemporalId equal to i.
sub_layer_level_idc[ i ] indicates the level value for the sub-layer representation with TemporalId equal to i.
In this bitstream, sub_layer_level_present_flag[i] is TRUE for PTL 1 & PTL 2. sub_layer_level_idc[ i ] value is as below.
- vps_max_sub_layers_minus1: 2
- profile_tier_level() structure 0
   profilePresentFlag: 1, maxNumSubLayersMinus1: 2
    general_profile_idc: 1
    general_level_idc: 93
- profile_tier_level() structure 1
   profilePresentFlag: 0, maxNumSubLayersMinus1: 2
    general_level_idc: 93
    sub_layer_level_present_flag[ 0 ]: 1
    sub_layer_level_present_flag[ 1 ]: 1
    sub_layer_level_idc[ 0 ]: 90
    sub_layer_level_idc[ 1 ]: 90
- profile_tier_level() structure 2
   profilePresentFlag: 1, maxNumSubLayersMinus1: 2
    general_profile_idc: 7
    general_level_idc: 123
    sub_layer_level_present_flag[ 0 ]: 1
    sub_layer_level_present_flag[ 1 ]: 1
    sub_layer_level_idc[ 0 ]: 120
    sub_layer_level_idc[ 1 ]: 120
- Number of Frames - 8

Output Layer Sets:
- OLS_0 - Layer: 0, PTL Idx: 1 (Main 3.1, 3)
- OLS_1 - Layer: 0, PTL Idx: 1 (Main 3.1, 3)
          Layer: 1, PTL Idx: 2 (Scalable Main 4.1, 4)

Number of layers: 2

Max number of Temporal sub-layers: 3

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3.1
              Sub-layer 0 level: 3
              Sub-layer 1 level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4.1
              Sub-layer 0 level: 4
              Sub-layer 1 level: 4

Each layer resolution:
- Layer 0 resolution - Coded:  640x360
- Layer 1 resolution - Coded:  1280x720

Frame rate: 60 fps for both layers

This bitstream is compatible with SHM rev 1025

Contact: Jill Boyce, Vidyo Inc. (jill@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

