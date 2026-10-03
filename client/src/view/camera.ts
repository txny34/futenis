import * as THREE from "three";

// Camera behind the human (team 0, looking down +X). "Fixed" is a stable
// broadcast-ish framing. "Dynamic" is a sports-game over-the-shoulder view:
// shoulder height, close behind and slightly to the right of the human,
// exponentially damped. The look target sits far down the court and leans
// toward the ball so the ball, the rival and the rival back glass stay framed.
const FOV = 62;
const NEAR = 0.1;
const FAR = 200;
const FIXED_POS = { x: -15.5, y: 6.2, z: 0 } as const;
const FIXED_LOOK = { x: 1.5, y: 0.6, z: 0 } as const;
const BACK_OFFSET = 5.5; // meters behind the human
const HEIGHT = 4.0; // high enough to look down on the court (~20° pitch) like sports games
const SHOULDER_Z = 0; // centered: the human sits in the lower third, court fills the width
const MIN_X = -16; // may sit outside the near glass (rendered almost invisible)
const MAX_X = -4; // never past this toward the net
const FOLLOW_Z = 0.85; // fraction of the human's Z the camera copies
const LOOK_AHEAD_X = 7; // look target this far ahead of the human...
const LOOK_MIN_X = 1; // ...but always at least at the net
const LOOK_BALL_X = 0.3;
const LOOK_BALL_Z = 0.3;
const LOOK_HUMAN_Z = 0.5;
const LOOK_Y = 0;
const LOOK_BALL_Y = 0.2; // high balls tilt the camera up a little
const POS_RATE = 4; // 1/s damping
const LOOK_RATE = 5;
const SHAKE_DURATION_S = 0.25;
const SHAKE_AMPLITUDE = 0.1;
const SHAKE_FREQ_A = 53;
const SHAKE_FREQ_B = 71;

const damp = (rate: number, dt: number): number => 1 - Math.exp(-rate * dt);

export class CameraRig {
  readonly camera = new THREE.PerspectiveCamera(FOV, 1, NEAR, FAR);
  dynamic = true;
  private readonly pos = new THREE.Vector3(FIXED_POS.x, FIXED_POS.y, FIXED_POS.z);
  private readonly look = new THREE.Vector3(FIXED_LOOK.x, FIXED_LOOK.y, FIXED_LOOK.z);
  private readonly goalPos = new THREE.Vector3();
  private readonly goalLook = new THREE.Vector3();
  private shakeLeft = 0;
  private time = 0;

  constructor() { this.apply(); }

  shake(): void { this.shakeLeft = SHAKE_DURATION_S; }

  /** human = interpolated feet (null if no human on court); ball = ball center. */
  update(human: THREE.Vector3 | null, ball: THREE.Vector3, dt: number): void {
    this.time += dt;
    if (this.dynamic && human) {
      this.goalPos.set(Math.min(MAX_X, Math.max(MIN_X, human.x - BACK_OFFSET)), HEIGHT, human.z * FOLLOW_Z + SHOULDER_Z);
      this.goalLook.set(
        Math.max(LOOK_MIN_X, human.x + LOOK_AHEAD_X) + ball.x * LOOK_BALL_X,
        LOOK_Y + ball.y * LOOK_BALL_Y,
        human.z * LOOK_HUMAN_Z + ball.z * LOOK_BALL_Z,
      );
    } else {
      this.goalPos.set(FIXED_POS.x, FIXED_POS.y, FIXED_POS.z);
      this.goalLook.set(FIXED_LOOK.x, FIXED_LOOK.y, FIXED_LOOK.z);
    }
    this.pos.lerp(this.goalPos, damp(POS_RATE, dt));
    this.look.lerp(this.goalLook, damp(LOOK_RATE, dt));
    this.apply();
    this.applyShake(dt);
  }

  resize(aspect: number): void {
    this.camera.aspect = aspect;
    this.camera.updateProjectionMatrix();
  }

  private apply(): void {
    this.camera.position.copy(this.pos);
    this.camera.lookAt(this.look);
  }

  private applyShake(dt: number): void {
    if (this.shakeLeft <= 0) return;
    this.shakeLeft = Math.max(0, this.shakeLeft - dt);
    const k = SHAKE_AMPLITUDE * (this.shakeLeft / SHAKE_DURATION_S);
    this.camera.position.y += Math.sin(this.time * SHAKE_FREQ_A) * k;
    this.camera.position.z += Math.sin(this.time * SHAKE_FREQ_B) * k;
  }
}
