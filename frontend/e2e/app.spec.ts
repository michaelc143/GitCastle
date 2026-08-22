import { test, expect } from '@playwright/test'

test('creates a repository from the workspace', async ({ page }) => {
  await page.route('**/api/v1/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: 1, username: 'alice', created_at: new Date().toISOString() }),
    })
  })

  await page.route('**/api/v1/repositories', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ repositories: [] }) })
      return
    }
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ id: 1, owner: 'alice', name: 'castle', path: '/repos/alice/castle.git', created_at: new Date().toISOString() }),
    })
  })

  await page.goto('/')
  await expect(page.getByText('No repositories yet. Build your first stronghold above.')).toBeVisible()
  await page.getByLabel('Repository name').fill('castle')
  await page.getByRole('button', { name: 'Create repository' }).click()
  await expect(page).toHaveURL(/#\/alice\/castle$/)
  await expect(page.getByRole('heading', { name: 'alice/castle', exact: true })).toBeVisible()
})
