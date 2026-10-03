import * as THREE from "three";
import { THEME } from "./theme";

// Gradient night dome. One draw, no lighting, never writes depth.
const SKY_RADIUS = 160;
const SKY_SEGMENTS = 24;
const SKY_RINGS = 12;
const HORIZON_SHARPNESS = 0.55; // < 1 widens the horizon glow band
const FOG_DENSITY = 0.012;

const VERT = /* glsl */ `
varying float vH;
void main() {
  vH = normalize(position).y;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;

const FRAG = /* glsl */ `
uniform vec3 uTop;
uniform vec3 uHorizon;
uniform float uSharp;
varying float vH;
void main() {
  float t = pow(clamp(vH, 0.0, 1.0), uSharp);
  gl_FragColor = vec4(mix(uHorizon, uTop, t), 1.0);
}`;

export function buildSky(): THREE.Mesh {
  const mat = new THREE.ShaderMaterial({
    vertexShader: VERT,
    fragmentShader: FRAG,
    uniforms: {
      uTop: { value: new THREE.Color(THEME.skyTop) },
      uHorizon: { value: new THREE.Color(THEME.skyHorizon) },
      uSharp: { value: HORIZON_SHARPNESS },
    },
    side: THREE.BackSide,
    depthWrite: false,
    fog: false,
  });
  const sky = new THREE.Mesh(new THREE.SphereGeometry(SKY_RADIUS, SKY_SEGMENTS, SKY_RINGS), mat);
  sky.renderOrder = -1;
  sky.frustumCulled = false;
  return sky;
}

export function buildFog(): THREE.FogExp2 {
  return new THREE.FogExp2(THEME.fog, FOG_DENSITY);
}
