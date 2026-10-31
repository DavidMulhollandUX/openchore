import { describe, it, expect } from 'vitest';
import type { Meal } from '../types';
import { mealsByDay, recipeParagraphs, tonight, weekDays } from './Meals.data';

const meal = (id: number, date: string, kind: Meal['meal'], title = `Meal ${id}`): Meal => ({ id, date, meal: kind, title });

describe('weekDays', () => {
  it('lists today and the next six days, across a month end', () => {
    expect(weekDays(new Date(2026, 9, 29))).toEqual([
      '2026-10-29', '2026-10-30', '2026-10-31', '2026-11-01', '2026-11-02', '2026-11-03', '2026-11-04',
    ]);
  });
});

describe('mealsByDay', () => {
  it('groups by day in the order a day runs, and drops days outside the range', () => {
    const days = ['2026-11-02', '2026-11-03'];
    const byDay = mealsByDay([
      meal(1, '2026-11-02', 'dinner'),
      meal(2, '2026-11-02', 'breakfast'),
      meal(3, '2026-11-09', 'lunch'),
    ], days);
    expect(byDay.get('2026-11-02')!.map(m => m.id)).toEqual([2, 1]);
    expect(byDay.get('2026-11-03')).toEqual([]);
    expect(byDay.has('2026-11-09')).toBe(false);
  });
});

describe('tonight', () => {
  it("prefers today's dinner", () => {
    const meals = [meal(1, '2026-11-02', 'lunch'), meal(2, '2026-11-02', 'dinner'), meal(3, '2026-11-03', 'dinner')];
    expect(tonight(meals, '2026-11-02').map(m => m.id)).toEqual([2]);
  });

  it('falls back to whatever today has', () => {
    expect(tonight([meal(1, '2026-11-02', 'lunch')], '2026-11-02').map(m => m.id)).toEqual([1]);
    expect(tonight([], '2026-11-02')).toEqual([]);
  });
});

describe('recipeParagraphs', () => {
  it('keeps bold and italic, link words, and splits on blank lines', () => {
    expect(recipeParagraphs('Brown the **mince**, then *rest*.\n\nSee [my tips](https://x.test) ![pic](a.png)')).toEqual([
      [{ text: 'Brown the ' }, { text: 'mince', strong: true }, { text: ', then ' }, { text: 'rest', em: true }, { text: '.' }],
      [{ text: 'See ' }, { text: 'my tips' }, { text: ' ' }],
    ]);
  });

  it('joins wrapped lines and drops heading and list marks', () => {
    expect(recipeParagraphs('## Prep\r\n- chop the onion\n  finely\n\n\n')).toEqual([
      [{ text: 'Prep chop the onion finely' }],
    ]);
  });

  it('leaves plain text alone and never makes HTML', () => {
    expect(recipeParagraphs('Heat to 200°C <b>hot</b>')).toEqual([[{ text: 'Heat to 200°C <b>hot</b>' }]]);
    expect(recipeParagraphs('   ')).toEqual([]);
  });
});
