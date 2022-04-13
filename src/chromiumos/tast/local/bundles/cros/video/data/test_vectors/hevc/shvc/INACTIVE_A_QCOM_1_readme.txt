Bitstream file name
-------------------
INACTIVE_A_QCOM_1.txt

Bitstream feature name
----------------------
Setting default_ref_layers_active_flag to zero

Explanation of bitstream features
---------------------------------
This bitstream is coded using the RA configuration, with three layers; layer 0 is a direct reference layer of layer 1, and layers 0 and 1 are direct reference layers of layer 2. The value of default_ref_layers_active_flag is set equal to 0 in the VPS, and hence the syntax elements inter_layer_pred_enabled_flag, num_inter_layer_ref_pics_minus1, and inter_layer_pred_layer_idc[ i ] are signalled in the slice segment header. The layers are SNR scalable layers, and all layers have the same resolution.

Number of output layer sets in the bitstream
--------------------------------------------
3

Number of layers present in the bitstream
-----------------------------------------
3

Profile, tier, and level (P,T,L) for each layer in each output layer set in the bitstream
-----------------------------------------------------------------------------------------
OLS 0 - L0: (main, main, 2.1)
	
OLS 1 - L0: (main, main, 2.1)
        L1: (scalable-main, main, 2.1)

OLS 2 - L0: (main, main, 2.1)
        L1: (scalable-main, main, 2.1)
        L2: (scalable-main, main, 2.1)

Picture size of each layer
--------------------------
L0: 416 x 240
L1: 416 x 240
L2: 416 x 240

Frame rate of each layer (if available)
---------------------------------------
L0: 50 fps
L1: 50 fps
L2: 50 fps

SHM version number used to generate the bitstream, if appropriate
-----------------------------------------------------------------
SHM-dev branch, r1025

Number of frames
----------------
100

Contact
-------
Adarsh K. Ramasubramonian (aramasub@qti.qualcomm.com)
