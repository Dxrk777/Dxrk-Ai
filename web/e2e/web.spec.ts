import { test, expect } from '@playwright/test'

test.describe('Navigation', () => {
  test('redirects / to /dashboard', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/dashboard/)
  })

  test('sidebar shows DXRK branding', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('aside')).toContainText('DXRK')
    await expect(page.locator('aside')).toContainText('.ai')
  })

  test('sidebar has all nav links', async ({ page }) => {
    await page.goto('/dashboard')
    const nav = page.locator('aside nav')
    await expect(nav.getByText('DASHBOARD')).toBeVisible()
    await expect(nav.getByText('LOGS')).toBeVisible()
    await expect(nav.getByText('AGENTS')).toBeVisible()
    await expect(nav.getByText('SETTINGS')).toBeVisible()
  })

  test('active nav link is highlighted', async ({ page }) => {
    await page.goto('/dashboard')
    const dashboardLink = page.locator('aside nav a[href="/dashboard"]')
    await expect(dashboardLink).toHaveClass(/cyber-cyan/)
  })

  test('navigates to logs page', async ({ page }) => {
    await page.goto('/dashboard')
    await page.click('aside nav a[href="/logs"]')
    await expect(page).toHaveURL(/\/logs/)
    await expect(page.locator('h1')).toContainText('LIVE LOGS')
  })

  test('navigates to agents page', async ({ page }) => {
    await page.goto('/dashboard')
    await page.click('aside nav a[href="/agents"]')
    await expect(page).toHaveURL(/\/agents/)
    await expect(page.locator('h1')).toContainText('AGENT POOL')
  })

  test('navigates to settings page', async ({ page }) => {
    await page.goto('/dashboard')
    await page.click('aside nav a[href="/settings"]')
    await expect(page).toHaveURL(/\/settings/)
    await expect(page.locator('h1')).toContainText('CONFIGURATION')
  })

  test('unknown route redirects to dashboard', async ({ page }) => {
    await page.goto('/nonexistent')
    await expect(page).toHaveURL(/\/dashboard/)
  })
})

test.describe('Dashboard', () => {
  test('shows DXRK.AI CORE header', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('h1')).toContainText('DXRK.AI CORE')
  })

  test('shows stat cards grid', async ({ page }) => {
    await page.goto('/dashboard')
    const cards = page.locator('.grid.grid-cols-4 > div')
    await expect(cards).toHaveCount(4)
    await expect(cards.nth(0)).toContainText('AGENT STATUS')
    await expect(cards.nth(1)).toContainText('PROVIDERS')
    await expect(cards.nth(2)).toContainText('COST')
    await expect(cards.nth(3)).toContainText('VECTORS')
  })

  test('shows live logs section', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('text=LIVE LOGS')).toBeVisible()
    await expect(page.locator('text=Waiting for logs...')).toBeVisible()
  })

  test('shows provider pool section', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('text=PROVIDER POOL')).toBeVisible()
  })

  test('shows connection status indicator', async ({ page }) => {
    await page.goto('/dashboard')
    const online = page.locator('header span:has-text("ONLINE")')
    const offline = page.locator('header span:has-text("OFFLINE")')
    await expect(online.or(offline)).toBeVisible()
  })

  test('shows version in sidebar footer', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.locator('aside')).toContainText('v4.0.0')
    await expect(page.locator('aside')).toContainText('AGENT ACTIVE')
  })
})

test.describe('LogsPage', () => {
  test('shows log level filter buttons', async ({ page }) => {
    await page.goto('/logs')
    for (const level of ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG']) {
      await expect(page.getByRole('button', { name: level })).toBeVisible()
    }
  })

  test('shows pause and clear buttons', async ({ page }) => {
    await page.goto('/logs')
    await expect(page.locator('text=CLEAR')).toBeVisible()
    await expect(page.locator('.lucide-pause, .lucide-play')).toBeVisible()
  })

  test('filter buttons are clickable', async ({ page }) => {
    await page.goto('/logs')
    const errorBtn = page.getByRole('button', { name: 'ERROR' })
    await errorBtn.click()
    await expect(errorBtn).toHaveClass(/cyber-cyan/)
  })

  test('shows waiting state when no logs', async ({ page }) => {
    await page.goto('/logs')
    await expect(page.locator('text=Waiting for log data...')).toBeVisible()
  })

  test('shows connection status', async ({ page }) => {
    await page.goto('/logs')
    const connected = page.locator('p:has-text("CONNECTED")')
    const disconnected = page.locator('p:has-text("DISCONNECTED")')
    await expect(connected.or(disconnected)).toBeVisible()
  })
})

test.describe('AgentsPage', () => {
  test('shows AGENT POOL header', async ({ page }) => {
    await page.goto('/agents')
    await expect(page.locator('h1')).toContainText('AGENT POOL')
  })

  test('shows refresh and spawn buttons', async ({ page }) => {
    await page.goto('/agents')
    await expect(page.locator('text=REFRESH')).toBeVisible()
    await expect(page.locator('text=SPAWN')).toBeVisible()
  })

  test('shows loading or empty state', async ({ page }) => {
    await page.goto('/agents')
    const loading = page.locator('text=Loading providers...')
    const empty = page.locator('text=No providers configured')
    await expect(loading.or(empty)).toBeVisible()
  })
})

test.describe('SettingsPage', () => {
  test('shows CONFIGURATION header', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.locator('h1')).toContainText('CONFIGURATION')
  })

  test('shows save and reset buttons', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.locator('text=SAVE')).toBeVisible()
    await expect(page.locator('text=RESET')).toBeVisible()
  })

  test('save button is disabled when not dirty', async ({ page }) => {
    await page.goto('/settings')
    const saveBtn = page.locator('button:has-text("SAVE")')
    await expect(saveBtn).toBeDisabled()
  })

  test('shows settings sections', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.getByRole('heading', { name: 'System' })).toBeVisible()
  })
})
