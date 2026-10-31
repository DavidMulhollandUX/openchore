import type { Meal, MealType } from '../types';
import { localDateStr } from '../utils';

/** Meal types in the order a day runs. */
export const MEAL_ORDER: readonly MealType[] = ['breakfast', 'lunch', 'snack', 'dinner', 'side', 'dessert', 'drink'];

/** Today and the six days after it, as YYYY-MM-DD. */
export function weekDays(today: Date): string[] {
  const days: string[] = [];
  for (let i = 0; i < 7; i++) {
    const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() + i);
    days.push(localDateStr(d));
  }
  return days;
}

/** Meals for each of `days`, each day's in MEAL_ORDER. */
export function mealsByDay(meals: Meal[], days: string[]): Map<string, Meal[]> {
  const byDay = new Map<string, Meal[]>(days.map(d => [d, []]));
  for (const m of meals) byDay.get(m.date)?.push(m);
  for (const list of byDay.values()) {
    list.sort((a, b) => MEAL_ORDER.indexOf(a.meal) - MEAL_ORDER.indexOf(b.meal) || a.id - b.id);
  }
  return byDay;
}

/** What "Tonight" shows: today's dinner, else whatever today has. */
export function tonight(meals: Meal[], today: string): Meal[] {
  const todays = mealsByDay(meals, [today]).get(today) ?? [];
  const dinner = todays.filter(m => m.meal === 'dinner');
  return dinner.length > 0 ? dinner : todays;
}

/** A YYYY-MM-DD string as a local Date. */
export function parseDay(day: string): Date {
  const [y, m, d] = day.split('-').map(Number);
  return new Date(y, m - 1, d);
}

/** A run of recipe text, bold or italic or neither. */
export interface TextRun {
  text: string;
  strong?: boolean;
  em?: boolean;
}

const INLINE = /\*\*(.+?)\*\*|__(.+?)__|\*(.+?)\*|_(.+?)_|!?\[([^\]]*)\]\([^)]*\)/g;

/** Mealie's recipe text is Markdown. Recipe pages show it as plain
 *  paragraphs: bold and italic stay, a link keeps its words and drops the
 *  address (a tap shouldn't leave the tablet), an image is dropped, and
 *  heading and list marks go. Nothing here becomes HTML. */
export function recipeParagraphs(text: string): TextRun[][] {
  return text
    .replace(/\r\n?/g, '\n')
    .split(/\n\s*\n/)
    .map(block => block
      .split('\n')
      .map(line => line.trim().replace(/^(#{1,6}\s+|[-*+]\s+|>\s?)/, ''))
      .filter(Boolean)
      .join(' '))
    .filter(Boolean)
    .map(paragraph => {
      const runs: TextRun[] = [];
      let last = 0;
      for (const m of paragraph.matchAll(INLINE)) {
        if (m.index! > last) runs.push({ text: paragraph.slice(last, m.index) });
        const [whole, b1, b2, i1, i2, link] = m;
        if (b1 ?? b2) runs.push({ text: (b1 ?? b2)!, strong: true });
        else if (i1 ?? i2) runs.push({ text: (i1 ?? i2)!, em: true });
        else if (!whole.startsWith('!') && link) runs.push({ text: link });
        last = m.index! + whole.length;
      }
      if (last < paragraph.length) runs.push({ text: paragraph.slice(last) });
      return runs;
    })
    .filter(runs => runs.length > 0);
}
