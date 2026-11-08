const fs = require('fs');
const path = require('path');
const { Resvg } = require('@resvg/resvg-js');

function buildLogoSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <defs>
    <clipPath id="squircle-clip">
      <rect x="24" y="24" width="976" height="976" rx="220" />
    </clipPath>

    <!-- Electric Cyan Glowing Optics -->
    <linearGradient id="cyan-glow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#38bdf8"/>
      <stop offset="50%" stop-color="#00f5ff"/>
      <stop offset="100%" stop-color="#0284c7"/>
    </linearGradient>

    <linearGradient id="cyan-accent" x1="0%" y1="100%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#0284c7"/>
      <stop offset="100%" stop-color="#00f5ff"/>
    </linearGradient>

    <!-- Arctic Leopard Fur Gradients -->
    <linearGradient id="fur-white" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#ffffff"/>
      <stop offset="100%" stop-color="#e2e8f0"/>
    </linearGradient>

    <linearGradient id="fur-highlight" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#f8fafc"/>
      <stop offset="100%" stop-color="#cbd5e1"/>
    </linearGradient>

    <linearGradient id="fur-silver" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#cbd5e1"/>
      <stop offset="100%" stop-color="#94a3b8"/>
    </linearGradient>

    <linearGradient id="fur-slate" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#94a3b8"/>
      <stop offset="100%" stop-color="#64748b"/>
    </linearGradient>

    <!-- Volumetric Obsidian Armor Shading for Torso / Mantle -->
    <linearGradient id="armor-light" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#64748b"/>
      <stop offset="100%" stop-color="#475569"/>
    </linearGradient>

    <linearGradient id="armor-mid" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#475569"/>
      <stop offset="100%" stop-color="#334155"/>
    </linearGradient>

    <linearGradient id="armor-dark" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#334155"/>
      <stop offset="100%" stop-color="#1e293b"/>
    </linearGradient>

    <linearGradient id="armor-shadow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1e293b"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>

    <linearGradient id="armor-deep" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#020617"/>
    </linearGradient>

    <filter id="gateway-shadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="16" stdDeviation="22" flood-color="#0f172a" flood-opacity="0.16" />
    </filter>
  </defs>

  <!-- Luxury White Squircle Container -->
  <rect x="24" y="24" width="976" height="976" rx="220" fill="#ffffff" stroke="#e2e8f0" stroke-width="6" />

  <g clip-path="url(#squircle-clip)">
    <g transform="translate(512, 512)" filter="url(#gateway-shadow)">

      <!-- Hexagonal Architectural Gateway Frame (Distribution Spec Gateway) -->
      <polygon points="
        0,-380
        329,-190
        329,190
        0,380
        -329,190
        -329,-190
      " fill="none" stroke="#0f172a" stroke-width="36" stroke-linejoin="round" />

      <!-- Inner Precision Telemetry Dash -->
      <polygon points="
        0,-345
        298,-172
        298,172
        0,345
        -298,172
        -298,-172
      " fill="none" stroke="#00f5ff" stroke-width="4" opacity="0.45" stroke-dasharray="16, 12" />

      <!-- 100% Watertight Solid Base Silhouette (Arctic Snow Leopard Felid Contour) -->
      <path d="
        M 0 -220
        L 70 -210
        L 110 -195
        L 210 -255
        L 225 -155
        L 190 -85
        L 245 0
        L 195 85
        L 225 180
        L 250 255
        L 175 315
        L 85 335
        L 0 345
        L -85 335
        L -175 315
        L -250 255
        L -225 180
        L -195 85
        L -245 0
        L -190 -85
        L -225 -155
        L -210 -255
        L -110 -195
        L -70 -210
        Z
      " fill="#0b0f19" />

      <!-- Armored Mantle & Layer Stack Carapace (Below Neck, y >= 130) -->
      <!-- Tier 3: Bottom Collar & Chest Anchor -->
      <polygon points="0,345 85,335 175,315 250,255 160,225 0,250" fill="url(#armor-deep)" />
      <polygon points="0,345 -85,335 -175,315 -250,255 -160,225 0,250" fill="url(#armor-shadow)" />

      <!-- Tier 2: Mid Shoulder Flanks -->
      <polygon points="0,250 160,225 250,255 225,180 145,145 0,175" fill="url(#armor-shadow)" />
      <polygon points="0,250 -160,225 -250,255 -225,180 -145,145 0,175" fill="url(#armor-dark)" />

      <!-- Tier 1: Upper Neck Guard Plates -->
      <polygon points="0,175 145,145 225,180 195,85 120,95 0,130" fill="url(#armor-mid)" />
      <polygon points="0,175 -145,145 -225,180 -195,85 -120,95 0,130" fill="url(#armor-light)" />

      <!-- Alert Snow Leopard Ears (Wide Feline Angle, Tufted Inner Fur) -->
      <!-- Right Ear Outer -->
      <polygon points="110,-195 210,-255 225,-155" fill="url(#fur-slate)" />
      <polygon points="110,-195 225,-155 190,-85 135,-140" fill="url(#fur-silver)" />
      <!-- Right Ear Inner Cavity (Frost Fur + Cyan Glow) -->
      <polygon points="125,-190 198,-235 205,-165 145,-150" fill="url(#fur-silver)" />
      <polygon points="135,-180 188,-220 192,-170" fill="url(#cyan-glow)" opacity="0.6" />

      <!-- Left Ear Outer -->
      <polygon points="-110,-195 -210,-255 -225,-155" fill="url(#fur-slate)" />
      <polygon points="-110,-195 -225,-155 -190,-85 -135,-140" fill="url(#fur-silver)" />
      <!-- Left Ear Inner Cavity -->
      <polygon points="-125,-190 -198,-235 -205,-165 -145,-150" fill="url(#fur-white)" />
      <polygon points="-135,-180 -188,-220 -192,-170" fill="url(#cyan-glow)" opacity="0.6" />

      <!-- Forehead & Cranial Dome (Pure Arctic Snow Leopard Origami Planes) -->
      <polygon points="0,-220 70,-210 110,-195 65,-150 0,-165" fill="url(#fur-white)" />
      <polygon points="0,-220 -70,-210 -110,-195 -65,-150 -0,-165" fill="url(#fur-highlight)" />

      <!-- Central Forehead Diamond -->
      <polygon points="0,-165 65,-150 0,-95" fill="url(#fur-white)" />
      <polygon points="0,-165 -65,-150 0,-95" fill="url(#fur-highlight)" />

      <!-- Upper Temple Planes -->
      <polygon points="65,-150 110,-195 135,-140 75,-85" fill="url(#fur-highlight)" />
      <polygon points="-65,-150 -110,-195 -135,-140 -75,-85" fill="url(#fur-silver)" />

      <polygon points="135,-140 190,-85 150,-25 75,-85" fill="url(#fur-silver)" />
      <polygon points="-135,-140 -190,-85 -150,-25 -75,-85" fill="url(#fur-slate)" />

      <!-- Wide Zygomatic Big-Cat Cheekbones (Silver & Slate Fur Planes) -->
      <!-- Right Cheek -->
      <polygon points="150,-25 190,-85 245,0 175,35" fill="url(#fur-highlight)" />
      <polygon points="95,-15 150,-25 175,35 85,45" fill="url(#fur-silver)" />
      <polygon points="175,35 245,0 195,85 120,95 85,80" fill="url(#fur-silver)" />
      <!-- Left Cheek -->
      <polygon points="-150,-25 -190,-85 -245,0 -175,35" fill="url(#fur-silver)" />
      <polygon points="-95,-15 -150,-25 -175,35 -85,45" fill="url(#fur-slate)" />
      <polygon points="-175,35 -245,0 -195,85 -120,95 -85,80" fill="url(#fur-slate)" />

      <!-- Subtle Leopard Rosettes on Temples -->
      <polygon points="85,-125 115,-120 102,-102 78,-108" fill="#475569" opacity="0.35" />
      <polygon points="-85,-125 -115,-120 -102,-102 -78,-108" fill="#475569" opacity="0.35" />
      <polygon points="180,-30 210,-25 195,-5 170,-10" fill="#475569" opacity="0.35" />
      <polygon points="-180,-30 -210,-25 -195,-5 -170,-10" fill="#475569" opacity="0.35" />

      <!-- Brow Ridges & Eye Contour -->
      <polygon points="0,-95 65,-150 75,-85 0,-60" fill="url(#fur-highlight)" />
      <polygon points="0,-95 -65,-150 -75,-85 0,-60" fill="url(#fur-silver)" />

      <!-- Nose Bridge (Sleek Tapering Down to Rhinarium) -->
      <polygon points="0,-60 45,-62 30,-5 0,15" fill="url(#fur-white)" />
      <polygon points="0,-60 -45,-62 -30,-5 0,15" fill="url(#fur-highlight)" />

      <!-- Sub-Orbital Fur Cheek Plates (Clean Frost Titanium) -->
      <polygon points="45,-62 75,-85 150,-25 95,-15 30,-5" fill="url(#fur-white)" />
      <polygon points="-45,-62 -75,-85 -150,-25 -95,-15 -30,-5" fill="url(#fur-highlight)" />

      <!-- Electric Cyan Almond Predator Eyes (Sharp Feline Upward Cant) -->
      <!-- Right Eye Socket -->
      <polygon points="50,-60 105,-50 115,-34 60,-42" fill="#1e293b" />
      <polygon points="55,-56 100,-47 108,-37 64,-44" fill="url(#cyan-glow)" />
      <!-- Vertical Slit Pupil / Core Highlight -->
      <polygon points="80,-52 86,-51 84,-40 78,-41" fill="#020617" />
      <polygon points="84,-52 96,-50 92,-44 82,-46" fill="#ffffff" opacity="0.9" />

      <!-- Left Eye Socket -->
      <polygon points="-50,-60 -105,-50 -115,-34 -60,-42" fill="#1e293b" />
      <polygon points="-55,-56 -100,-47 -108,-37 -64,-44" fill="url(#cyan-glow)" />
      <!-- Left Pupil / Glint -->
      <polygon points="-80,-52 -86,-51 -84,-40 -78,-41" fill="#020617" />
      <polygon points="-84,-52 -96,-50 -92,-44 -82,-46" fill="#ffffff" opacity="0.9" />

      <!-- Feline Rhinarium (Dark Charcoal Origami Nose with Cyan Accent) -->
      <polygon points="0,15 28,15 16,40 0,46 -16,40 -28,15" fill="#0b0f19" stroke="#00f5ff" stroke-width="1.8" stroke-linejoin="round" />

      <!-- Whisker Pads & Muzzle (Refined Titanium Slate, Lateral Spread, Clean Cat Profile) -->
      <!-- Right Muzzle Pad -->
      <polygon points="0,46 16,40 28,15 30,-5 95,-15 85,45 28,78 0,80" fill="url(#fur-white)" />
      <polygon points="28,78 85,45 120,95 85,80" fill="url(#fur-silver)" />

      <!-- Left Muzzle Pad -->
      <polygon points="0,46 -16,40 -28,15 -30,-5 -95,-15 -85,45 -28,78 -0,80" fill="url(#fur-highlight)" />
      <polygon points="-28,78 -85,45 -120,95 -85,80" fill="url(#fur-silver)" />

      <!-- Lower Chin & Jaw Plate -->
      <polygon points="0,80 28,78 16,125 0,130" fill="url(#fur-highlight)" />
      <polygon points="0,80 -28,78 -16,125 0,130" fill="url(#fur-silver)" />

      <!-- Central Hexagonal OCI Layer Cache Core (Mantle Heart) -->
      <!-- Outer Hexagon -->
      <polygon points="
        0,185
        32,203
        32,239
        0,257
        -32,239
        -32,203
      " fill="#0b0f19" stroke="url(#cyan-accent)" stroke-width="5" stroke-linejoin="round" />

      <!-- High-Speed OCI Layer Cache Stack Bars -->
      <rect x="-18" y="201" width="36" height="4" rx="2" fill="url(#cyan-glow)" />
      <rect x="-18" y="211" width="36" height="4" rx="2" fill="url(#cyan-glow)" />
      <rect x="-18" y="221" width="36" height="4" rx="2" fill="url(#cyan-glow)" />
      <polygon points="0,231 -8,241 8,241" fill="#00f5ff" />

    </g>
  </g>
</svg>`;
}

async function renderLogo(outputDir) {
  const svg = buildLogoSvg();
  const svgPath = path.join(outputDir, 'logo.svg');
  const pngPath = path.join(outputDir, 'logo.png');

  fs.writeFileSync(svgPath, svg, 'utf-8');
  console.log('✓ Written logo.svg');

  const resvg = new Resvg(svg, {
    fitTo: { mode: 'width', value: 1024 },
    shapeRendering: 2,
    textRendering: 1,
    imageRendering: 0,
  });
  const pngData = resvg.render();
  const pngBuffer = pngData.asPng();
  fs.writeFileSync(pngPath, pngBuffer);
  console.log('✓ Rendered logo.png at 1024x1024 (' + pngBuffer.length + ' bytes)');
}

const outDir = path.resolve(__dirname, '../docs/images');
renderLogo(outDir).catch(err => {
  console.error(err);
  process.exit(1);
});
