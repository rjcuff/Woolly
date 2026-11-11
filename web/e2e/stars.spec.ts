import { expect, test } from '@playwright/test';
import { formatStars } from '../app/lib/github';

test('star counts are formatted like 186.7K', () => {
  expect(formatStars(5)).toBe('5');
  expect(formatStars(999)).toBe('999');
  expect(formatStars(1234)).toBe('1.2K');
  expect(formatStars(186_700)).toBe('186.7K');
  expect(formatStars(18_670)).toBe('18.7K');
  expect(formatStars(2_000)).toBe('2K');
  expect(formatStars(1_500_000)).toBe('1.5M');
});
