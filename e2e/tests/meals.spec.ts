import { test, expect, type Page } from '@playwright/test';
import { enterPin } from './helpers/setup';

// The suite has no Mealie, so the browser's /api/meals calls are answered
// here. The server side (talking to Mealie, who may change the plan) is
// covered by internal/api/meals_test.go.

function localDay(offset = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

async function fakeMeals(page: Page) {
  const meals = [
    { id: 1, date: localDay(), meal: 'dinner', title: 'Tacos', total_time: '30 min', recipe_slug: 'tacos' },
    { id: 2, date: localDay(1), meal: 'lunch', title: 'Leftovers' },
  ];
  const list = {
    id: 'l1', name: 'Groceries', items: [
      { id: 'i1', label: '2 avocados', checked: false },
      { id: 'i2', label: 'Milk', checked: false },
    ],
  };
  const sent: { method: string; path: string; body: unknown }[] = [];
  await page.route('**/api/meals/**', async route => {
    const req = route.request();
    const path = new URL(req.url()).pathname;
    const body = req.postData() ? JSON.parse(req.postData()!) : null;
    sent.push({ method: req.method(), path, body });
    if (path === '/api/meals/status') return route.fulfill({ json: { enabled: true, mealie_url: 'http://mealie.test' } });
    if (path === '/api/meals/plan' && req.method() === 'GET') return route.fulfill({ json: meals });
    if (path === '/api/meals/shopping') return route.fulfill({ json: list });
    if (path.startsWith('/api/meals/shopping/items/')) return route.fulfill({ status: 204 });
    if (path === '/api/meals/recipes') return route.fulfill({ json: [{ id: 'r9', name: 'Pumpkin soup', slug: 'pumpkin-soup' }] });
    if (path === '/api/meals/plan' && req.method() === 'POST') {
      const added = { id: 3, date: (body as { date: string }).date, meal: 'dinner', title: 'Pumpkin soup' };
      meals.push(added);
      return route.fulfill({ status: 201, json: added });
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });
  return sent;
}

test.describe('Meals', () => {
  test('opens from the picker, ticks the shopping list and goes back', async ({ page }) => {
    const sent = await fakeMeals(page);
    await page.goto('/login');
    await page.getByRole('button', { name: 'Meals', exact: true }).click();
    await expect(page).toHaveURL('/meals');

    await expect(page.getByRole('heading', { name: 'Tacos', exact: true })).toBeVisible();
    await expect(page.getByText('Tonight: Tacos')).toBeVisible();

    await page.getByRole('button', { name: 'Week' }).click();
    await expect(page.getByText('Leftovers')).toBeVisible();
    // Signed out, the week is read-only.
    await expect(page.getByRole('form', { name: 'Add a meal' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Remove / })).toHaveCount(0);

    await page.getByRole('button', { name: 'Shopping' }).click();
    await page.getByRole('button', { name: 'Tick off 2 avocados' }).click();
    await expect(page.getByRole('button', { name: 'Put 2 avocados back on the list' })).toBeVisible();
    expect(sent).toContainEqual({ method: 'PUT', path: '/api/meals/shopping/items/i1', body: { checked: true } });

    await page.getByRole('button', { name: "Who's here?" }).click();
    await expect(page).toHaveURL('/login');
  });

  test('a grown-up signs in from Meals to change the plan', async ({ page }) => {
    const sent = await fakeMeals(page);
    await page.goto('/meals');
    await page.getByRole('button', { name: 'Week' }).click();
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).toHaveURL('/login?next=/meals');

    await page.getByRole('button', { name: 'Select profile for Alex' }).click();
    await enterPin(page, '1234');
    await expect(page).toHaveURL('/meals');

    await page.getByRole('button', { name: 'Week' }).click();
    await page.getByLabel('Recipe or dish').fill('pump');
    await page.getByRole('button', { name: 'Pumpkin soup' }).click();
    await page.getByRole('button', { name: 'Add', exact: true }).click();
    await expect(page.getByText('Pumpkin soup')).toBeVisible();
    expect(sent).toContainEqual({ method: 'POST', path: '/api/meals/plan', body: { date: localDay(), meal: 'dinner', recipe_id: 'r9' } });
    await expect(page.getByRole('button', { name: 'Remove Tacos' })).toBeVisible();

    // Leaving signs the shared tablet out again.
    await page.getByRole('button', { name: "Who's here?" }).click();
    await expect(page).toHaveURL('/login');
    await page.goto('/meals');
    await page.getByRole('button', { name: 'Week' }).click();
    await expect(page.getByRole('button', { name: 'Remove Tacos' })).toHaveCount(0);
  });
});
