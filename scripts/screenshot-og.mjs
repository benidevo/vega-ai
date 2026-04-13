import puppeteer from "puppeteer-core";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");
const TEMPLATE = path.join(__dirname, "og-template.html");
const OUT_DIR = path.join(ROOT, "static", "landing", "assets", "images");

const OG_W = 1200;
const OG_H = 630;

function chromePath() {
  if (process.env.CHROME_PATH) return process.env.CHROME_PATH;
  if (process.platform === "darwin") {
    return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
  }
  for (const p of ["/usr/bin/google-chrome", "/usr/bin/chromium-browser", "/usr/bin/chromium"]) {
    if (fs.existsSync(p)) return p;
  }
  throw new Error("Chrome not found. Set the CHROME_PATH environment variable.");
}

async function capture() {
  const browser = await puppeteer.launch({
    headless: true,
    executablePath: chromePath(),
    args: ["--no-sandbox", "--disable-setuid-sandbox"],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: OG_W, height: OG_H, deviceScaleFactor: 2 });
  await page.goto(`file://${TEMPLATE}`, { waitUntil: "networkidle0" });
  await new Promise((r) => setTimeout(r, 1500)); // wait for Google Fonts to render

  fs.mkdirSync(OUT_DIR, { recursive: true });

  const ogPath = path.join(OUT_DIR, "og-image.png");
  const twitterPath = path.join(OUT_DIR, "twitter-card.png");

  await page.screenshot({ path: ogPath, type: "png", clip: { x: 0, y: 0, width: OG_W, height: OG_H } });
  fs.copyFileSync(ogPath, twitterPath);
  await browser.close();

  console.log(`✓ og-image.png     → ${ogPath}`);
  console.log(`✓ twitter-card.png → ${twitterPath}`);
}

capture().catch((err) => {
  console.error(err);
  process.exit(1);
});
