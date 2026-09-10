import { test, expect } from '@playwright/test';

test('static health is public and contains no application data', async ({ request }) => {
  const response = await request.get('/healthz');
  expect(response.status()).toBe(200);
  expect(await response.json()).toEqual({ status: 'ok' });
});
