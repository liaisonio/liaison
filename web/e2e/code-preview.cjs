const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const browser = await chromium.launch(
    process.env.CHROME_EXECUTABLE
      ? { executablePath: process.env.CHROME_EXECUTABLE }
      : undefined,
  );
  try {
    for (const locale of ['zh-CN', 'en-US'])
      for (const dark of [false, true])
        for (const width of [1280, 390]) {
          const page = await browser.newPage({
            viewport: { width, height: 900 },
            permissions: ['clipboard-read', 'clipboard-write'],
          });
          const errors = [],
            calls = [];
          const transfers = new Map();
          let serial = 0;
          const source =
            'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("<script>not executable</script>")\n}\n// ' +
            'long '.repeat(200) +
            '\n';
          const bytes = Buffer.from(source);
          const markdown = '# Project guide\n\n## Setup\n\n- Install dependencies\n- Start the service\n\n| Feature | Status |\n| --- | --- |\n| Preview | Ready |\n\n```bash\nnpm run dev\n```\n\n![Tracking image](https://example.invalid/tracker.png)\n\n<script>window.markdownExecuted=true</script>\n';
          page.on('pageerror', (e) => errors.push(e.message));
          await page.addInitScript(
            ({ locale, dark }) => {
              localStorage.setItem('liaison-locale', locale);
              document.addEventListener('DOMContentLoaded', () => {
                document.documentElement.className = dark ? 'dark' : 'light';
              });
            },
            { locale, dark },
          );
          await page.route('**/api/v1/edge-agents', async (route) => {
            const request = route.request().postDataJSON();
            calls.push(request);
            const f = request.file || {};
            let result = {
              version: 1,
              status: 'ok',
              running: false,
              closed: false,
            };
            if (request.action === 'transcript')
              result = { ...result, history_pages: [] };
            else if (request.action === 'file_cancel') {
              transfers.delete(f.transfer_id);
            } else if (request.action === 'file_list')
              result =
                f.path === 'src'
                  ? {
                      ...result,
                      files: [
                        {
                          name: 'main.go',
                          path: 'src/main.go',
                          size: bytes.length,
                        },
                      ],
                    }
                  : { ...result, status: 'invalid_request' };
            else if (request.action === 'file_read') {
              if (f.path === 'slow.go')
                await new Promise((r) => setTimeout(r, 500));
              const data =
                f.path === 'README.md' ? Buffer.from(markdown) : f.path === 'binary.dat' ? Buffer.from([0, 1, 2]) : bytes;
              if (
                f.path &&
                ![
                  'src/main.go',
                  'README.md',
                  'main.go',
                  'large.go',
                  'binary.dat',
                  'slow.go',
                ].includes(f.path)
              )
                result = { ...result, status: 'invalid_request' };
              else {
                let t = f.transfer_id
                  ? transfers.get(f.transfer_id)
                  : undefined;
                if (!f.transfer_id) {
                  t = { id: 'transfer-' + ++serial, data, path: f.path };
                  transfers.set(t.id, t);
                }
                if (!t) result = { ...result, status: 'invalid_request' };
                else {
                  const offset = f.offset || 0,
                    end = Math.min(t.data.length, offset + 70);
                  result = {
                    ...result,
                    transfer_id: t.id,
                    file_data: t.data.subarray(offset, end).toString('base64'),
                    file_offset: end,
                    file_done: end === t.data.length,
                    ...(!f.transfer_id
                      ? {
                          file: {
                            name: t.path.split('/').at(-1),
                            path: t.path,
                            size:
                              f.path === 'large.go'
                                ? 2 * 1024 * 1024
                                : t.data.length,
                          },
                        }
                      : {}),
                  };
                  if (end === t.data.length) transfers.delete(t.id);
                }
              }
            } else throw new Error('Unexpected action ' + request.action);
            await route.fulfill({ json: { data: result } });
          });
          const zh = locale === 'zh-CN',
            button = (name) => page.getByRole('button', { name, exact: true }),
            dialog = () => page.getByRole('dialog');
          await page.goto(
            (process.env.E2E_UI_URL || 'http://127.0.0.1:5303') +
              '/e2e/code-preview.html',
          );
          await button('Entry').waitFor();
          assert.equal(calls.length, 0, 'No automatic file reads');
          await button('README').click();
          await dialog().getByRole('heading', {name:'Project guide',exact:true}).waitFor();
          assert.equal(await dialog().locator('table').count(),1);
          assert.equal(await dialog().locator('img,script').count(),0);
          assert.equal(await page.evaluate(()=>window.markdownExecuted),undefined);
          await page.screenshot({path:`/tmp/readme-preview-${locale}-${dark}-${width}.png`});
          await dialog().getByRole('button',{name:zh?'源码':'Source',exact:true}).click();
          await dialog().locator('.agent-source-code').waitFor();
          await dialog().getByRole('button',{name:zh?'预览':'Preview',exact:true}).click();
          await dialog().getByRole('heading',{name:'Project guide',exact:true}).waitFor();
          await dialog().getByRole('button',{name:'Close',exact:true}).click();
          await button('README line').click();
          await dialog().locator('.agent-source-line.is-target').waitFor();
          assert.equal((await dialog().locator('.agent-source-line.is-target').innerText()).trim(),'3\n## Setup');
          await dialog().getByRole('button',{name:'Close',exact:true}).click();
          const parsed = await page.evaluate(() => [
            window.resolveFileLink(
              '/workspace/project/src/main.go:3:2',
              '/workspace/project',
            ),
            window.resolveFileLink(
              'file:///workspace/project/src/main.go#L3-L5',
              '/workspace/project',
            ),
            window.resolveFileLink('C:\\work\\app\\main.go:2', 'C:\\work\\app'),
            window.resolveFileLink(
              '/workspace/project-evil/main.go',
              '/workspace/project',
            ),
            window.resolveFileLink('%2e%2e/secret', '/workspace/project'),
            window.resolveFileLink(
              'file://remote/etc/passwd',
              '/workspace/project',
            ),
          ]);
          assert.deepEqual(parsed.slice(0, 3), [
            { path: 'src/main.go', line: 3, endLine: 3 },
            { path: 'src/main.go', line: 3, endLine: 5 },
            { path: 'main.go', line: 2, endLine: 2 },
          ]);
          assert(parsed.slice(3).every((v) => v == null));
          assert.equal(
            await page
              .getByRole('link', { name: 'Web', exact: true })
              .getAttribute('href'),
            'https://example.com',
          );
          assert.equal(await button('Unsafe').count(), 0);
          const activity = page.locator('.edge-agent-activity').last();
          assert(await activity.evaluate((e) => e.open));
          await button('Progress message').click();
          assert(
            await activity.evaluate((e) => e.open),
            'Earlier completed steps stay visible during this round',
          );
          await button('Complete').click();
          assert.equal(await activity.evaluate((e) => e.open), false);
          await activity.locator(':scope > summary').click();
          assert(
            await activity.evaluate((e) => e.open),
            'Manual expansion preserved',
          );
          await button('Next round').click();
          assert(
            await page
              .locator('.edge-agent-activity')
              .last()
              .evaluate((e) => e.open),
          );
          await page
            .locator('.edge-agent-activity')
            .last()
            .locator(':scope > summary')
            .click();
          await button('Progress message').click();
          assert.equal(
            await page
              .locator('.edge-agent-activity')
              .last()
              .evaluate((e) => e.open),
            false,
            'Manual collapse respected',
          );
          await button('Next round').click();
          assert(
            await page
              .locator('.edge-agent-activity')
              .last()
              .evaluate((e) => e.open),
            'Manual collapse does not leak into another round',
          );
          // Restrict links to the current round after the rollover retained older rounds.
          const link = (name) => button(name).last();
          await link('Entry').click();
          await page
            .locator('.agent-source-code .hljs-keyword')
            .first()
            .waitFor({ timeout: 10000 })
            .catch(async (error) => {
              await page.screenshot({ path: '/tmp/code-preview-error.png' });
              console.error(
                await dialog().innerText(),
                calls.slice(-3),
                errors,
              );
              throw error;
            });
          assert.equal(
            await page.locator('.agent-source-line.is-target').count(),
            3,
          );
          assert.equal(await dialog().locator('script').count(), 0);
          assert.equal(
            await page
              .locator('.agent-source-code code')
              .innerText()
              .then((t) => t.includes('<script>not executable</script>')),
            true,
          );
          await dialog()
            .getByRole('button', {
              name: zh ? '复制代码' : 'Copy code',
              exact: true,
            })
            .click();
          assert.equal(
            await page.evaluate(() => navigator.clipboard.readText()),
            source,
          );
          const download = page.waitForEvent('download');
          await dialog()
            .getByRole('button', {
              name: zh ? '下载' : 'Download',
              exact: true,
            })
            .click();
          const downloaded = await download;
          assert.equal(downloaded.suggestedFilename(), 'main.go');
          const chunks = [];
          for await (const chunk of await downloaded.createReadStream())
            chunks.push(chunk);
          assert.deepEqual(Buffer.concat(chunks), bytes);
          if (width > 700) {
            const separator = page.getByRole('separator');
            await separator.focus();
            await page.keyboard.press('ArrowLeft');
            assert.equal(await separator.getAttribute('aria-valuenow'), '680');
            const rect = await separator.boundingBox();
            await page.mouse.move(rect.x + 5, rect.y + 100);
            await page.mouse.down();
            await page.mouse.move(rect.x - 55, rect.y + 100);
            await page.mouse.up();
            assert.equal(await separator.getAttribute('aria-valuenow'), '740');
          }
          await page.screenshot({
            path: `/tmp/code-preview-${locale}-${dark}-${width}.png`,
            fullPage: true,
          });
          assert(
            await page.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth + 1,
            ),
          );
          await dialog()
            .getByRole('button', {
              name: zh ? '放大预览' : 'Expand preview',
              exact: true,
            })
            .click();
          assert.equal(
            Math.round(
              (await page.locator('.agent-code-preview-panel').boundingBox())
                .width,
            ),
            width,
          );
          await dialog()
            .getByRole('button', {
              name: zh ? '还原预览' : 'Restore preview',
              exact: true,
            })
            .click();
          await page.keyboard.press('Escape');
          await dialog().waitFor({ state: 'hidden' });
          assert(
            await link('Entry').evaluate((e) => e === document.activeElement),
            'Focus returns to the source link',
          );
          for (const name of ['Absolute', 'File URL', 'Colon']) {
            await link(name).click();
            await page
              .locator('.agent-source-line.is-target')
              .first()
              .waitFor();
            await page.keyboard.press('Escape');
            await dialog().waitFor({ state: 'hidden' });
          }
          await link('Directory').click();
          await dialog()
            .getByRole('button', { name: 'main.go', exact: true })
            .click();
          await page.locator('.agent-source-code').waitFor();
          await page.keyboard.press('Escape');
          await dialog().waitFor({ state: 'hidden' });
          for (const name of ['Outside', 'Traversal']) {
            const before = calls.length;
            await link(name).click();
            await dialog().locator('.liaison-notice.is-danger').waitFor();
            assert.equal(
              calls.length,
              before,
              'Invalid path must not request files',
            );
            await page.keyboard.press('Escape');
            await dialog().waitFor({ state: 'hidden' });
          }
          await link('Missing').click();
          await dialog().locator('.liaison-notice.is-danger').waitFor();
          assert.equal(
            await dialog()
              .getByRole('button', { name: zh ? '重试' : 'Retry', exact: true })
              .count(),
            1,
          );
          await page.keyboard.press('Escape');
          await dialog().waitFor({ state: 'hidden' });
          for (const name of ['Large', 'Binary']) {
            await link(name).click();
            await dialog().locator('.liaison-notice').waitFor();
            assert.equal(await page.locator('.agent-source-code').count(), 0);
            assert.equal(
              await dialog()
                .getByRole('button', {
                  name: zh ? '下载' : 'Download',
                  exact: true,
                })
                .count(),
              1,
            );
            await page.keyboard.press('Escape');
            await dialog().waitFor({ state: 'hidden' });
          }
          assert(
            calls.some((c) => c.action === 'file_cancel'),
            'Bounded preview cancels unfinished transfer',
          );
          await link('Slow').click();
          await page.keyboard.press('Escape');
          await dialog().waitFor({ state: 'hidden' });
          await page.waitForTimeout(650);
          assert.equal(
            await dialog().count(),
            0,
            'Late response cannot reopen preview',
          );
          await button('Offline').click();
          const before = calls.length;
          await link('Entry').click();
          await dialog().locator('.liaison-notice.is-danger').waitFor();
          assert.equal(calls.length, before);
          await page.keyboard.press('Escape');
          assert.deepEqual(errors, []);
          await page.close();
        }
    console.log(
      'PASS code preview: highlighting, paths, line ranges, directory navigation, resize, fullscreen, copy/download, bounded reads, offline/late response and activity lifecycle across 8 combinations',
    );
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
