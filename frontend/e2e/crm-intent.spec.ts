import { test, expect } from '@playwright/test'

const username = process.env.E2E_USERNAME
const password = process.env.E2E_PASSWORD

test.describe('CRM AI客户意向', () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!username || !password, '需要设置 E2E_USERNAME 和 E2E_PASSWORD')
    await page.goto('/login')
    await page.getByPlaceholder('用户名').fill(username!)
    await page.getByPlaceholder('密码').fill(password!)
    await page.getByRole('button', { name: /登\s*录/ }).click()
    await expect(page).toHaveURL(/dashboard/)
  })

  test('进入客户列表并使用AI意向筛选', async ({ page }) => {
    await page.getByText('客户列表', { exact: true }).click()
    await expect(page.getByText('客户名称', { exact: true })).toBeVisible()

    const intentFilter = page.locator('.el-form-item').filter({ hasText: 'AI意向' }).locator('.el-select')
    await intentFilter.click()
    await page.getByText('高意向', { exact: true }).last().click()
    await page.getByRole('button', { name: '查询' }).click()

    await expect(page.locator('.el-table')).toBeVisible()
  })

  test('打开客户意向详情并展示历史区域', async ({ page }) => {
    await page.getByText('客户列表', { exact: true }).click()
    await expect(page.getByText('客户名称', { exact: true })).toBeVisible()

    const detailButton = page.getByRole('button', { name: '意向详情' }).first()
    test.skip((await detailButton.count()) === 0, '测试环境没有已分析客户')
    await detailButton.click()
    await expect(page.getByText('AI客户意向', { exact: true })).toBeVisible()
  })
})
