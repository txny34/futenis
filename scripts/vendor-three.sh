#!/usr/bin/env bash
# Vendors three.js into client/vendor/three (served by cmd/dev; no CDN, no Node).
# Downloads the npm tarball, verifies its pinned sha512 and copies only the
# files the client imports (client/src/view/post.ts + transitive deps).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="0.169.0"
URL="https://registry.npmjs.org/three/-/three-${VERSION}.tgz"
INTEGRITY="Ed906MA3dR4TS5riErd4QBsRGPcx+HBDX2O5yYE5GqJeFQTPU+M56Va/f/Oph9X7uZo3W3o4l2ZhBZ6f6qUv0w=="
DEST="client/vendor/three"
FILES=(
  LICENSE
  build/three.module.js
  examples/jsm/postprocessing/EffectComposer.js
  examples/jsm/postprocessing/MaskPass.js
  examples/jsm/postprocessing/OutputPass.js
  examples/jsm/postprocessing/Pass.js
  examples/jsm/postprocessing/RenderPass.js
  examples/jsm/postprocessing/ShaderPass.js
  examples/jsm/postprocessing/UnrealBloomPass.js
  examples/jsm/shaders/CopyShader.js
  examples/jsm/shaders/LuminosityHighPassShader.js
  examples/jsm/shaders/OutputShader.js
)

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$URL" -o "$tmp/three.tgz"
got=$(openssl dgst -sha512 -binary "$tmp/three.tgz" | openssl base64 -A)
if [ "$got" != "$INTEGRITY" ]; then
  echo "three ${VERSION}: sha512 mismatch (got $got)" >&2
  exit 1
fi
tar -xzf "$tmp/three.tgz" -C "$tmp"
rm -rf "$DEST"
for f in "${FILES[@]}"; do
  mkdir -p "$DEST/$(dirname "$f")"
  cp "$tmp/package/$f" "$DEST/$f"
done
echo "three ${VERSION} vendored into $DEST (${#FILES[@]} files)"
