import { test, expect } from '@playwright/test'

const username = process.env.E2E_USERNAME
const password = process.env.E2E_PASSWORD

async function openMenuItem(page: import('@playwright/test').Page, parent: string, child: string) {
  await page.locator('.el-sub-menu__title').filter({ hasText: new RegExp(`^${parent}$`) }).click()
  await page.getByText(child, { exact: true }).click()
}

test.describe('AI运营与分析流程', () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!username || !password, '需要设置 E2E_USERNAME 和 E2E_PASSWORD')
    await page.goto('/login')
    await page.getByPlaceholder('用户名').fill(username!)
    await page.getByPlaceholder('密码').fill(password!)
    await page.getByRole('button', { name: /登\s*录/ }).click()
    await expect(page).toHaveURL(/dashboard/)
  })

  test('查看AI运营指标与费用估算', async ({ page }) => {
    await openMenuItem(page, '客户管理', 'AI意向指标')
    await expect(page.getByText('AI意向指标', { exact: true }).last()).toBeVisible()
    await expect(page.getByText('请求数', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('预估总费用', { exact: true })).toBeVisible()
    await expect(page.getByText('模型调用分组', { exact: true })).toBeVisible()
  })

  test('会话令牌使用HttpOnly Cookie且可进入模型治理', async ({ page, context }) => {
    const cookies = await context.cookies()
    expect(cookies.find((cookie) => cookie.name === 'nanyicrm_access')?.httpOnly).toBeTruthy()
    expect(cookies.find((cookie) => cookie.name === 'nanyicrm_refresh')?.httpOnly).toBeTruthy()
    await openMenuItem(page, '系统管理', 'AI模型治理')
    await expect(page.getByText('候选配置必须通过固定样本质量门禁后才能灰度或激活。')).toBeVisible()
  })

  test('模型治理可以查看切换审计记录', async ({ page }) => {
    await openMenuItem(page, '系统管理', 'AI模型治理')
    await expect(page.locator('.el-table__body-wrapper tbody tr').first()).toBeVisible({ timeout: 15_000 })
    const auditButton = page.locator('.el-table').first().getByRole('button', { name: '审计记录' }).first()
    await auditButton.click()
    const dialog = page.locator('.el-dialog').filter({ hasText: /模型治理审计记录/ })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByText(/暂无审计记录|动作/).first()).toBeVisible()
  })

  test('草稿模型可以打开绑定凭证操作', async ({ page }) => {
    await openMenuItem(page, '系统管理', 'AI模型治理')
    await expect(page.getByRole('button', { name: '绑定凭证' }).first()).toBeVisible()
    await page.getByRole('button', { name: '绑定凭证' }).first().click()
    await expect(page.getByText('绑定AI凭证', { exact: true })).toBeVisible()
    await expect(page.getByText(/重新执行质量门禁/)).toBeVisible()
  })

  test('模型治理可以打开凭证管理且不回显明文', async ({ page }) => {
    await openMenuItem(page, '系统管理', 'AI模型治理')
    await page.getByRole('button', { name: '凭证管理' }).click()
    await expect(page.getByText('AI凭证管理', { exact: true })).toBeVisible()
    await expect(page.getByText('服务端加密存储，页面不会回显明文。')).toBeVisible()
  })

  test('Prompt治理可以查看版本与质量门禁入口', async ({ page }) => {
    await openMenuItem(page, '系统管理', 'AI模型治理')
    await expect(page.getByText('当前实际生效 Prompt', { exact: true })).toBeVisible()
    await expect(page.getByText('Prompt治理', { exact: true })).toBeVisible()
    await expect(page.getByText('动态配置仅承载研判规则，JSON 输出契约仍由代码固定校验。')).toBeVisible()
    await expect(page.getByText('回退Prompt版本', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: '新增Prompt版本' })).toBeVisible()
    await expect(page.getByRole('button', { name: '质量门禁' }).last()).toBeVisible()
  })

  test('执行客户AI分析并打开历史结果', async ({ page }) => {
    test.setTimeout(90_000)
    test.skip(!process.env.E2E_AI_ENABLED, '需要在可访问模型服务的环境设置 E2E_AI_ENABLED=true')
    await openMenuItem(page, '客户管理', '客户列表')
    await expect(page.locator('.el-table__header-wrapper').getByText('客户名称', { exact: true })).toBeVisible()
    await expect(page.locator('.el-table__body-wrapper tbody tr').first()).toBeVisible({ timeout: 15_000 })
    const analyze = page.getByRole('button', { name: 'AI分析', exact: true }).first()
    test.skip((await analyze.count()) === 0, '测试环境没有可分析客户')
    await analyze.click()
    await expect(page.getByText('AI客户意向', { exact: true })).toBeVisible({ timeout: 60_000 })
    await expect(page.getByText('分析历史', { exact: true })).toBeVisible()
  })
})
