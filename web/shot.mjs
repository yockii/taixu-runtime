// 渲染自测：vite dev（代理 /api → 本机 runtime:3000 真数据）+ Playwright(Edge) 截观察面板。
import { chromium } from 'playwright';
import { spawn } from 'node:child_process';
import { mkdirSync } from 'node:fs';

mkdirSync('shots', { recursive: true });
const srv = spawn('npx', ['vite', 'dev', '--port', '5199', '--strictPort'], { shell: true, stdio: 'pipe' });
await new Promise((res, rej) => {
  srv.stdout.on('data', (d) => { if (String(d).includes('5199')) res(); });
  srv.stderr.on('data', (d) => process.stderr.write(d));
  setTimeout(() => rej(new Error('dev 启动超时')), 20000);
});

const browser = await chromium.launch({ channel: 'msedge' });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
await page.goto('http://localhost:5199/', { waitUntil: 'networkidle' }).catch(() => {});
await page.waitForTimeout(2500); // 等 SSE 首批事件 + 入场动效
await page.screenshot({ path: 'shots/panel.png', fullPage: true });
// 切几个标签页截图
for (const [name, label] of [['goals', 1], ['dialogue', 2], ['reflections', 4]]) {
  await page.locator('.ttab').nth(label).click();
  await page.waitForTimeout(900);
  await page.screenshot({ path: `shots/panel-${name}.png`, fullPage: true });
}
// 移动端
await page.setViewportSize({ width: 390, height: 844 });
await page.goto('http://localhost:5199/', { waitUntil: 'networkidle' }).catch(() => {});
await page.waitForTimeout(1800);
await page.screenshot({ path: 'shots/m-panel.png', fullPage: true });
await browser.close();
srv.kill();
process.exit(0);
