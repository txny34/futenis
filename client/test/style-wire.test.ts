import { test } from "node:test";
import assert from "node:assert/strict";
import { DEFAULT_STYLE, CharacterStyle } from "../src/character/character";
import { STYLES_MSG_LEN, STYLE_LEN, decodeLook, decodeStyles, encodeStyle } from "../src/net/style-wire";
import { HEADER } from "../src/net/protocol";

const MINE: CharacterStyle = {
  ...DEFAULT_STYLE, hair: "afro", headwear: "beanie", skin: 3, hairColor: 2, hatColor: 4, number: 23,
  eyewear: "shades", eyewearColor: 2, facialHair: "beard", wristbands: true, wristColor: 3, bootsColor: 4,
};

function stylesWith(slot: number, look: Uint8Array): Uint8Array {
  const msg = new Uint8Array(STYLES_MSG_LEN);
  msg.set(look.subarray(HEADER), HEADER + slot * STYLE_LEN);
  return msg;
}

test("a look round-trips with its shirt number", () => {
  const look = decodeLook(encodeStyle(MINE), HEADER, 0);
  assert.ok(look);
  assert.equal(look.hair, "afro");
  assert.equal(look.number, 23);
  assert.equal(look.headwear, "beanie");
  assert.equal(look.eyewear, "shades");
  assert.equal(look.eyewearColor, 2);
  assert.equal(look.facialHair, "beard");
  assert.equal(look.wristbands, true);
  assert.equal(look.wristColor, 3);
  assert.equal(look.bootsColor, 4);
  assert.ok(Math.abs(look.headSize - MINE.headSize) < 0.01);
});

test("STYLES maps server slot s to view slot s ^ me", () => {
  const msg = stylesWith(2, encodeStyle(MINE));
  for (const me of [0, 1, 2, 3]) {
    const looks = decodeStyles(msg, msg.length, me);
    assert.ok(looks);
    assert.equal(looks[2 ^ me]?.hair, "afro", `me=${me}`);
    assert.equal(looks.filter((l) => l !== null).length, 1);
  }
});

test("untrusted bytes are sanitized", () => {
  const msg = new Uint8Array(HEADER + STYLE_LEN).fill(255);
  const look = decodeLook(msg, HEADER, 7);
  assert.ok(look);
  assert.equal(look.hair, DEFAULT_STYLE.hair);
  assert.ok(look.number >= 0 && look.number <= 99);
  assert.equal(decodeLook(new Uint8Array(HEADER + STYLE_LEN), HEADER, 7), null); // absent
});

test("an old client's look (no number) falls back to the seat number", () => {
  const v1 = encodeStyle(MINE);
  v1[HEADER + 8] = 0;
  assert.equal(decodeLook(v1, HEADER, 9)?.number, 9);
  assert.equal(decodeStyles(new Uint8Array(STYLES_MSG_LEN - 1), STYLES_MSG_LEN - 1, 0), null);
});
