import { BODY } from "./config";

// Head landmarks shared by the dressing (parts.ts) and the face (face.ts), in
// head-bone space. The head sphere is scaled for the toy look: wider and
// deeper, never taller (the 1.80 m hitbox caps the height).

export const HEAD_SCALE_XZ = 1.15;
export const HEAD_SCALE_Y = 1.0;
export const HEAD_Y = BODY.headCenter - BODY.headBase;
export const FACE_Z = BODY.headRadius * HEAD_SCALE_XZ * 0.93;
export const EYE_X = 0.055;
export const EYE_Y = HEAD_Y + 0.015;
export const EYE_R = 0.03;
export const BROW_Y = EYE_Y + 0.04;
export const MOUTH_Y = HEAD_Y - 0.055;
