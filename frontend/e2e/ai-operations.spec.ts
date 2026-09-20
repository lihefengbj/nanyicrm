import { test, expect } from '@playwright/test'

const username = process.env.E2E_USERNAME
const password = process.env.E2E_PASSWORD

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
    await page.getByText('AI意向指标', { exact: true }).click()
    await expect(page.getByText('AI意向运营指标', { exact: true })).toBeVisible()
    await expect(page.getByText('请求次数', { exact: true })).toBeVisible()
    await expect(page.getByText('估算费用', { exact: true })).toBeVisible()
    await expect(page.getByText('按模型与版本', { exact: true })).toBeVisible()
  })

  test('会话令牌使用HttpOnly Cookie且可进入模型治理', async ({ page, context }) => {
    const cookies = await context.cookies()
    expect(cookies.find((cookie) => cookie.name === 'nanyicrm_access')?.httpOnly).toBeTruthy()
    expect(cookies.find((cookie) => cookie.name === 'nanyicrm_refresh')?.httpOnly).toBeTruthy()
    await page.getByText('AI模型治理', { exact: true }).click()
    await expect(page.getByText('候选配置必须通过固定样本质量门禁后才能灰度或激活。')).toBeVisible()
  })

  test('执行客户AI分析并打开历史结果', async ({ page }) => {
    test.skip(!process.env.E2E_AI_ENABLED, '需要在可访问模型服务的环境设置 E2E_AI_ENABLED=true')
    await page.getByText('客户列表', { exact: true }).click()
    await expect(page.getByText('客户名称', { exact: true })).toBeVisible()
    const analyze = page.getByRole('button', { name: 'AI分析' }).first()
    test.skip((await analyze.count()) === 0, '测试环境没有可分析客户')
    await analyze.click()
    await expect(page.getByText('AI客户意向', { exact: true })).toBeVisible({ timeout: 60_000 })
    await expect(page.getByText('分析历史', { exact: true })).toBeVisible()
  })
})
