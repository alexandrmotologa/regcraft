const puppeteer = require('B:/workgit/node_modules/puppeteer-core');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const OUTPUT_DIR = path.resolve(__dirname, '../docs/images');
const FRAMES_DIR = path.resolve(__dirname, '../docs/images/frames');
const BASE_URL = 'http://localhost:5000';
const CHROME_PATH = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

async function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function run() {
  if (!fs.existsSync(OUTPUT_DIR)) {
    fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  }
  if (!fs.existsSync(FRAMES_DIR)) {
    fs.mkdirSync(FRAMES_DIR, { recursive: true });
  }

  console.log('Launching Chrome with puppeteer-core...');
  const browser = await puppeteer.launch({
    headless: 'new',
    executablePath: CHROME_PATH,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--window-size=1440,900'],
  });

  // 1. Desktop Overview (1440x900 @ 2x)
  console.log('1. Capturing Desktop Overview (regcraft_overview.png)...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(2000);
    await page.screenshot({ path: path.join(OUTPUT_DIR, 'regcraft_overview.png') });
    console.log('✓ regcraft_overview.png captured');
    await page.close();
  }

  // 2. Layer Visualizer Modal
  console.log('2. Capturing Layer Visualizer Modal (regcraft_layers.png)...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(1500);

    // Click the inspect button (first image in catalog)
    const inspectBtn = await page.$('button[title*="Inspect Layers"]');
    if (inspectBtn) {
      await inspectBtn.click();
      await sleep(1200);
    }
    await page.screenshot({ path: path.join(OUTPUT_DIR, 'regcraft_layers.png') });
    console.log('✓ regcraft_layers.png captured');
    await page.close();
  }

  // 3. Pre-Warm Modal
  console.log('3. Capturing Pre-Warm Modal (regcraft_warm.png)...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(1500);

    const warmBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll('button'));
      return btns.find(b => b.textContent && b.textContent.includes('Warm Image'));
    });
    if (warmBtn) {
      await warmBtn.click();
      await sleep(800);
      const input = await page.$('input[placeholder*="image name"]');
      if (input) {
        await input.type('library/golang:1.24-alpine', { delay: 40 });
      }
      await sleep(600);
    }
    await page.screenshot({ path: path.join(OUTPUT_DIR, 'regcraft_warm.png') });
    console.log('✓ regcraft_warm.png captured');
    await page.close();
  }

  // 4. Prune / GC Console Modal
  console.log('4. Capturing Prune Console Modal (regcraft_prune.png)...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(1500);

    const pruneBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll('button'));
      return btns.find(b => b.textContent && b.textContent.trim().startsWith('Prune'));
    });
    if (pruneBtn) {
      await pruneBtn.click();
      await sleep(1000);
    }
    await page.screenshot({ path: path.join(OUTPUT_DIR, 'regcraft_prune.png') });
    console.log('✓ regcraft_prune.png captured');
    await page.close();
  }

  // 5. Mobile Viewport (390x844 iPhone @ 2x)
  console.log('5. Capturing Mobile Viewport (regcraft_mobile.png)...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(2000);
    await page.screenshot({ path: path.join(OUTPUT_DIR, 'regcraft_mobile.png') });
    console.log('✓ regcraft_mobile.png captured');
    await page.close();
  }

  // 6. Record Animated Demo GIF frames
  console.log('6. Recording Animated Demo GIF frames...');
  {
    const page = await browser.newPage();
    await page.setViewport({ width: 1280, height: 800, deviceScaleFactor: 1 });
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await sleep(1500);

    let frameCount = 0;
    async function snap(times = 1, delay = 220) {
      for (let i = 0; i < times; i++) {
        const framePath = path.join(FRAMES_DIR, `frame_${String(frameCount++).padStart(4, '0')}.png`);
        await page.screenshot({ path: framePath });
        await sleep(delay);
      }
    }

    // Scene 1: Dashboard overview with live telemetry
    console.log('   GIF Scene 1: Overview...');
    await snap(7, 250);

    // Scene 2: Open Warm Image Modal & type
    console.log('   GIF Scene 2: Warm Image Modal...');
    const warmBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll('button'));
      return btns.find(b => b.textContent && b.textContent.includes('Warm Image'));
    });
    if (warmBtn) {
      await warmBtn.click();
      await snap(4, 200);
      const input = await page.$('input[placeholder*="image name"]');
      if (input) {
        await input.type('library/postgres:16', { delay: 60 });
        await snap(4, 200);
      }
    }
    await page.keyboard.press('Escape');
    await sleep(400);
    await snap(2, 200);

    // Scene 3: Open Layer Visualizer
    console.log('   GIF Scene 3: Inspect Layers Modal...');
    const inspectBtn = await page.$('button[title*="Inspect Layers"]');
    if (inspectBtn) {
      await inspectBtn.click();
      await snap(8, 250);
    }
    await page.keyboard.press('Escape');
    await sleep(400);
    await snap(3, 200);

    // Scene 4: Toggle Pin / Freeze Tag
    console.log('   GIF Scene 4: Toggle Pin Freeze...');
    const pinBtns = await page.$$('button[title*="Freeze Tag"], button[title*="Unfreeze Tag"]');
    if (pinBtns.length > 0) {
      await pinBtns[0].click();
      await snap(6, 250);
    }

    // Scene 5: Open Prune Console
    console.log('   GIF Scene 5: Prune Console...');
    const pruneBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll('button'));
      return btns.find(b => b.textContent && b.textContent.trim().startsWith('Prune'));
    });
    if (pruneBtn) {
      await pruneBtn.click();
      await snap(8, 250);
    }
    await page.keyboard.press('Escape');
    await sleep(400);
    await snap(4, 250);

    await page.close();
    console.log(`✓ Captured ${frameCount} frames for demo GIF.`);
  }

  await browser.close();

  // 7. Compile animated GIF using FFmpeg
  console.log('7. Compiling animated GIF with FFmpeg...');
  const palettePath = path.join(OUTPUT_DIR, 'palette.png');
  const gifPath = path.join(OUTPUT_DIR, 'regcraft_demo.gif');

  const ffmpegCmd1 = `ffmpeg -y -framerate 4 -i "${FRAMES_DIR}/frame_%04d.png" -vf "fps=4,scale=1000:-1:flags=lanczos,palettegen" "${palettePath}"`;
  execSync(ffmpegCmd1, { stdio: 'inherit' });

  const ffmpegCmd2 = `ffmpeg -y -framerate 4 -i "${FRAMES_DIR}/frame_%04d.png" -i "${palettePath}" -lavfi "fps=4,scale=1000:-1:flags=lanczos [x]; [x][1:v] paletteuse" "${gifPath}"`;
  execSync(ffmpegCmd2, { stdio: 'inherit' });

  // Cleanup frames and temporary palette
  fs.rmSync(FRAMES_DIR, { recursive: true, force: true });
  if (fs.existsSync(palettePath)) fs.unlinkSync(palettePath);

  console.log(`✓ Successfully compiled animated demo GIF: ${gifPath}`);
}

run().catch((err) => {
  console.error('Screenshot capture failed:', err);
  process.exit(1);
});
