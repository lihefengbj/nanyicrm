import { test, expect } from '@playwright/test'

const username = process.env.E2E_USERNAME
const password = process.env.E2E_PASSWORD

async function openMenuItem(page: import('@playwright/test').Page, parent: string, child: string) {
  await page.locator('.el-sub-menu__title').filter({ hasText: new RegExp(`^${parent}$`) }).click()
  await page.getByText(child, { exact: true }).click()
}

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
    await openMenuItem(page, '客户管理', '客户列表')
    await expect(page.locator('.el-table__header-wrapper').getByText('客户名称', { exact: true })).toBeVisible()

    const detailButton = page.getByRole('button', { name: '意向详情' }).first()
    test.skip((await detailButton.count()) === 0, '测试环境没有已分析客户')
    await detailButton.click()
    await expect(page.getByText('AI客户意向', { exact: true })).toBeVisible()
    await expect(page.getByText(/Prompt版本：/).first()).toBeVisible()
  })

  test('批量分析任务展示进度并支持取消', async ({ page }) => {
    await openMenuItem(page, '客户管理', '客户列表')
    await expect(page.locator('.el-table__header-wrapper').getByText('客户名称', { exact: true })).toBeVisible()
    const firstRow = page.locator('.el-table__body-wrapper tbody tr').first()
    test.skip((await firstRow.count()) === 0, '测试环境没有客户数据')

    let task = {
      id: 900001,
      tenantId: 1,
      createdBy: 1,
      status: 'pending',
      totalCount: 1,
      pendingCount: 1,
      runningCount: 0,
      successCount: 0,
      failedCount: 0,
      canceledCount: 0,
      maxAttempts: 3,
      errorMessage: '',
      createdAt: new Date().toISOString(),
      items: [{ id: 910001, taskId: 900001, customerId: 1, status: 'pending', attempts: 0, errorMessage: '' }],
    }
    await page.route('**/api/v1/crm/customer/intent/batch', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: task }) })
    })
    await page.route('**/api/v1/crm/customer/intent/tasks/900001', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: task }) })
    })
    await page.route('**/api/v1/crm/customer/intent/tasks/900001/cancel', async (route) => {
      task = { ...task, status: 'canceled', pendingCount: 0, canceledCount: 1, items: [{ ...task.items[0], status: 'canceled' }] }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: task }) })
    })

    await firstRow.locator('.el-checkbox').click()
    await page.getByRole('button', { name: /批量AI分析/ }).click()
    await page.getByRole('button', { name: /确认/ }).last().click()
    await expect(page.getByText('批量AI分析任务', { exact: true })).toBeVisible()
    await expect(page.getByText('任务 #900001', { exact: true })).toBeVisible()
    await expect(page.getByText('排队中 1', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: '取消未开始任务' }).click()
    await page.getByRole('button', { name: /确\s*定/ }).last().click()
    await expect(page.getByText('已取消', { exact: true })).toBeVisible()
  })

  test('人工反馈提交后在意向详情中回显', async ({ page }) => {
    let feedbackRows: unknown[] = []
    const customer = {
      id: 1,
      tenantId: 1,
      name: 'E2E意向客户',
      phone: '',
      source: '测试',
      industry: '企业服务',
      level: 'B',
      status: 1,
      ownerId: 1,
      intent: {
        id: 11,
        customerId: 1,
        analysisId: 101,
        intentLevel: 'medium',
        effectiveIntentLevel: 'medium',
        intentScore: 62,
        confidence: 0.8,
        summary: '客户正在了解方案',
        nextAction: '安排产品演示',
        suggestedNextAt: null,
        analyzedAt: new Date().toISOString(),
        provider: 'test',
        model: 'test-model',
        status: 'success',
        manualOverride: false,
        manualIntentLevel: '',
        followUpStatus: 'none',
      },
    }
    const result = {
      ...customer.intent,
      needs: ['客户管理'],
      painPoints: ['资料分散'],
      budget: '未明确',
      purchaseTimeline: '未明确',
      decisionRole: '业务负责人',
      risks: [],
    }
    const history = {
      id: 101,
      customerId: 1,
      triggerUserId: 1,
      result,
      status: 'success',
      errorMessage: '',
      provider: 'test',
      model: 'test-model',
      actualModel: 'test-model',
      modelConfigVersion: 'test-v1',
      adapterVersion: 'test-adapter',
      promptVersion: 'p-test',
      inputTokens: 10,
      outputTokens: 10,
      totalTokens: 20,
      providerRequestId: 'req-test',
      errorType: '',
      inputHash: 'hash',
      costMillis: 10,
      inputSummary: '客户：E2E意向客户',
      createdAt: new Date().toISOString(),
    }
    await page.route('**/api/v1/crm/customer?**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: { records: [customer], total: 1, pageNum: 1, pageSize: 10 } }) })
    })
    await page.route('**/api/v1/crm/customer/1/intent', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: result }) })
    })
    await page.route('**/api/v1/crm/customer/1/intent/history**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: { records: [history], total: 1, pageNum: 1, pageSize: 10 } }) })
    })
    await page.route('**/api/v1/crm/customer/1/intent/compare', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: { current: history, previous: null, scoreDiff: null, levelChanged: false } }) })
    })
    await page.route('**/api/v1/crm/customer/1/intent/feedback**', async (route) => {
      if (route.request().method() === 'POST') {
        feedbackRows = [{
          id: 201,
          customerId: 1,
          analysisId: 101,
          userId: 1,
          userName: 'E2E用户',
          feedbackType: 'accurate',
          accepted: true,
          manualIntentLevel: '',
          note: '建议有效',
          analysisAt: history.createdAt,
          createdAt: new Date().toISOString(),
        }]
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: { id: 201, appliedToCurrent: false } }) })
        return
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'success', data: { records: feedbackRows, total: feedbackRows.length, pageNum: 1, pageSize: 10 } }) })
    })

    await openMenuItem(page, '客户管理', '客户列表')
    await page.getByRole('button', { name: '意向详情' }).click()
    await expect(page.getByText('人工反馈', { exact: true })).toBeVisible()
    await page.getByPlaceholder('备注（可选）').fill('建议有效')
    await page.getByRole('button', { name: '提交反馈' }).click()
    await expect(page.getByText(/最近反馈：准确/)).toBeVisible()
    await expect(page.getByText(/备注：建议有效/)).toBeVisible()
  })
})
