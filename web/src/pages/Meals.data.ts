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
