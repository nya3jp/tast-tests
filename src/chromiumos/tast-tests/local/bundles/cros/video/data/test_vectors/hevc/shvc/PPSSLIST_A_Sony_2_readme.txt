PPSSLIST_A_Sony

Specification: All slices are coded as I, P or B slices. Each picture contains one slice.
vps_max_layers_minus1 is equal to 1. pps_infer_scaling_list_flag for the enhancement layer is equal to 1.

Functional stage: Test PPS scaling list inferring

Level: 4.1

Description:
The bitstream has two layers (See LX below for the coding width and height).
EL PPS scaling list is inferred from BL PPS scaling list.

Number of layers: 2

L0: 960x540
L1: 1280x720

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 4.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 4.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4.1)

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 4.1
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4.1

SHM Version: SHM Dev branch, rev 1015 (Trac ticket #59)

