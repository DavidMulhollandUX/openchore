import React, { useCallback, useEffect, useId, useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import type { Meal, MealRecipe, MealType, MealsStatus, RecipeCard, ShoppingList } from '../types';
import { localDateStr } from '../utils';
import { Button, ChoreRow, HouseScope, Icon, useMinuteClock, type IconName } from '../design';
import { BrandMark } from '../components/BrandMark/BrandMark';
import { MEAL_ORDER, mealsByDay, parseDay, tonight, weekDays } from './Meals.data';
import forms from '../styles/forms.module.css';
import styles from './Meals.module.css';

type Tab = 'tonight' | 'week' | 'recipes' | 'shopping';

const TAB_ICONS: Record<Tab, IconName> = { tonight: 'moon', week: 'cal', recipes: 'book', shopping: 'check' };
const isTab = (v: string | null): v is Tab => v != null && v in TAB_ICONS;
const REFRESH_MS = 60_000;

/** Where a meal's recipe opens, when it has one. */
const recipePath = (slug: string) => `/meals/recipes/${encodeURIComponent(slug)}`;

/** One planned meal: a chore row's well and text, without a check. A meal
 *  made from a recipe opens it. */
function MealRow({ meal, onRemove, busy }: { meal: Meal; onRemove?: () => void; busy?: boolean }) {
  const { t } = useTranslation();
  const meta = [t(`meals.type.${meal.meal}`), meal.total_time].filter(Boolean).join(' · ');
  const text = (
    <>
      <span className="oc-chore__title">{meal.title}</span>
      <span className="oc-chore__meta"><span>{meta}</span></span>
    </>
  );
  return (
    <div className={clsx('oc-chore', 'oc-choreCard', styles.meal)} data-cat="daily">
      <span className="oc-chore__well"><Icon name="dish" /></span>
      {meal.recipe_slug ? (
        <Link to={recipePath(meal.recipe_slug)} className={clsx('oc-chore__txt', styles.mealLink)}>
          {text}
        </Link>
      ) : (
        <span className="oc-chore__txt">{text}</span>
      )}
      {onRemove && (
        <button
          type="button"
          className={styles.remove}
          onClick={onRemove}
          disabled={busy}
          aria-label={t('meals.remove', { title: meal.title })}
        >
          {t('meals.removeShort')}
        </button>
      )}
    </div>
  );
}

/** A parent's form for adding a meal: a Mealie recipe, or just a name. */
function AddMeal({ days, onAdded }: { days: string[]; onAdded: () => void }) {
  const { t, i18n } = useTranslation();
  const [date, setDate] = useState(days[0]);
  const [kind, setKind] = useState<MealType>('dinner');
  const [text, setText] = useState('');
  const [recipe, setRecipe] = useState<MealRecipe | null>(null);
  const [matches, setMatches] = useState<MealRecipe[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const fieldId = useId();

  // The week moves on at midnight; don't keep a day it no longer shows.
  useEffect(() => {
    if (!days.includes(date)) setDate(days[0]);
  }, [days, date]);

  // Suggest recipes as the parent types; a pick is sent as the recipe.
  useEffect(() => {
    const q = text.trim();
    if (recipe || q.length < 2) { setMatches([]); return; }
    let current = true;
    const timer = setTimeout(() => {
      api.meals.recipes(q)
        .then(found => { if (current) setMatches(found); })
        .catch(() => { if (current) setMatches([]); });
    }, 250);
    return () => { current = false; clearTimeout(timer); };
  }, [text, recipe]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!recipe && !text.trim()) return;
    setBusy(true);
    setError('');
    try {
      await api.meals.add(recipe ? { date, meal: kind, recipe_id: recipe.id } : { date, meal: kind, title: text.trim() });
      setText('');
      setRecipe(null);
      onAdded();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('meals.error'));
    } finally {
      setBusy(false);
    }
  };

  const locale = i18n.resolvedLanguage || undefined;
  return (
    <form className={styles.add} onSubmit={submit} aria-label={t('meals.addTitle')}>
      <h2 className={styles.label}>{t('meals.addTitle')}</h2>
      <div className={styles.addFields}>
        <div className={forms.formGroup}>
          <label className={forms.label} htmlFor={`${fieldId}-day`}>{t('meals.day')}</label>
          <select id={`${fieldId}-day`} className={forms.input} value={date} onChange={e => setDate(e.target.value)}>
            {days.map(d => (
              <option key={d} value={d}>{parseDay(d).toLocaleDateString(locale, { weekday: 'long', day: 'numeric', month: 'short' })}</option>
            ))}
          </select>
        </div>
        <div className={forms.formGroup}>
          <label className={forms.label} htmlFor={`${fieldId}-meal`}>{t('meals.meal')}</label>
          <select id={`${fieldId}-meal`} className={forms.input} value={kind} onChange={e => setKind(e.target.value as MealType)}>
            {MEAL_ORDER.map(m => <option key={m} value={m}>{t(`meals.type.${m}`)}</option>)}
          </select>
        </div>
        <div className={clsx(forms.formGroup, styles.what)}>
          <label className={forms.label} htmlFor={`${fieldId}-what`}>{t('meals.what')}</label>
          <input
            id={`${fieldId}-what`}
            className={forms.input}
            value={recipe ? recipe.name : text}
            onChange={e => { setRecipe(null); setText(e.target.value); }}
            placeholder={t('meals.whatPlaceholder')}
            autoComplete="off"
          />
        </div>
      </div>
      {matches.length > 0 && (
        <ul className={styles.matches} aria-label={t('meals.recipes')}>
          {matches.map(r => (
            <li key={r.id}>
              <button type="button" className={styles.match} onClick={() => { setRecipe(r); setMatches([]); }}>
                <Icon name="dish" />{r.name}
              </button>
            </li>
          ))}
        </ul>
      )}
      {error && <p className={styles.error} role="alert">{error}</p>}
      <Button type="submit" icon="plus" disabled={busy || (!recipe && !text.trim())}>{t('meals.add')}</Button>
    </form>
  );
}

const DEBOUNCE_MS = 250;

/** The household's recipes from Mealie, to browse and open. */
function RecipesTab({ canEdit, mealieUrl }: { canEdit: boolean; mealieUrl?: string }) {
  const { t } = useTranslation();
  const searchId = useId();
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState('');
  const [cards, setCards] = useState<RecipeCard[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const timer = setTimeout(() => setQuery(search.trim()), DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [search]);

  // A new search starts again from the first page.
  useEffect(() => {
    let current = true;
    setBusy(true);
    api.meals.cookbook(query, 1)
      .then(p => {
        if (!current) return;
        setCards(p.items);
        setTotal(p.total);
        setPage(1);
        setError('');
      })
      .catch(() => { if (current) setError(t('meals.error')); })
      .finally(() => { if (current) { setBusy(false); setLoaded(true); } });
    return () => { current = false; };
  }, [query, t]);

  const more = async () => {
    setBusy(true);
    try {
      const p = await api.meals.cookbook(query, page + 1);
      setCards(c => [...c, ...p.items.filter(n => !c.some(o => o.id === n.id))]);
      setTotal(p.total);
      setPage(page + 1);
    } catch {
      setError(t('meals.error'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={styles.recipes} aria-label={t('meals.tabs.recipes')}>
      <div className={clsx(forms.formGroup, styles.search)}>
        <label className={forms.label} htmlFor={searchId}>{t('meals.searchRecipes')}</label>
        <input
          id={searchId}
          type="search"
          className={forms.input}
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder={t('meals.searchPlaceholder')}
          autoComplete="off"
        />
      </div>
      {error && <p className={styles.error} role="alert">{error}</p>}
      {loaded && cards.length === 0 && !error && (
        <p className={styles.empty}>{query ? t('meals.noRecipeMatch', { search: query }) : t('meals.noRecipes')}</p>
      )}
      <ul className={styles.cards}>
        {cards.map(r => (
          <li key={r.id}>
            <Link to={recipePath(r.slug)} className={styles.card}>
              {r.image
                ? <img className={styles.cardImage} src={r.image} alt="" loading="lazy" />
                : <span className={clsx(styles.cardImage, styles.cardWell)}><Icon name="dish" /></span>}
              <span className={styles.cardName}>{r.name}</span>
              {r.total_time && <span className={styles.cardMeta}><Icon name="clock" />{r.total_time}</span>}
            </Link>
          </li>
        ))}
      </ul>
      {cards.length < total && (
        <Button variant="quiet" onClick={more} disabled={busy}>{t('meals.moreRecipes')}</Button>
      )}
      {canEdit && mealieUrl && (
        <p className={styles.hint}>
          <Icon name="book" />
          <a className={styles.link} href={mealieUrl} target="_blank" rel="noopener noreferrer">{t('meals.editInMealie')}</a>
        </p>
      )}
    </section>
  );
}

export const Meals: React.FC = () => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { user, session, signOut } = useAuth();
  const clock = useMinuteClock();
  const canEdit = user?.role === 'admin';
  const persistent = session?.persistent ?? false;

  // The tab lives in the address, so coming back from a recipe lands on it.
  const [params, setParams] = useSearchParams();
  const tabParam = params.get('tab');
  const tab: Tab = isTab(tabParam) ? tabParam : 'tonight';
  const setTab = (id: Tab) => setParams(id === 'tonight' ? {} : { tab: id }, { replace: true });
  const [status, setStatus] = useState<MealsStatus | null>(null);
  const [meals, setMeals] = useState<Meal[]>([]);
  const [list, setList] = useState<ShoppingList | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState('');
  // Each row's write in flight, so one finishing doesn't free another.
  const [pending, setPending] = useState<ReadonlySet<string>>(new Set());
  const startPending = (key: string) => setPending(p => new Set(p).add(key));
  const endPending = (key: string) => setPending(p => { const n = new Set(p); n.delete(key); return n; });

  const today = localDateStr(clock);
  const days = useMemo(() => weekDays(parseDay(today)), [today]);

  const load = useCallback(async () => {
    try {
      const s = await api.meals.status();
      setStatus(s);
      if (!s.enabled) return;
      const [plan, shopping] = await Promise.all([
        api.meals.plan(days[0], days[days.length - 1]),
        api.meals.shopping(),
      ]);
      setMeals(plan);
      setList(shopping);
      setError('');
    } catch {
      setError(t('meals.error'));
    } finally {
      setLoaded(true);
    }
  }, [days, t]);

  useEffect(() => {
    load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, [load]);

  // Leaving signs a shared-device session out, so the next person at the
  // tablet can't change the plan.
  const back = async () => {
    if (user && !persistent) await signOut();
    navigate('/login');
  };

  const toggle = async (id: string, checked: boolean) => {
    startPending(id);
    setList(l => l && { ...l, items: l.items.map(it => it.id === id ? { ...it, checked } : it) });
    try {
      await api.meals.check(id, checked);
    } catch {
      setError(t('meals.error'));
      load();
    } finally {
      endPending(id);
    }
  };

  const remove = async (m: Meal) => {
    startPending(`meal-${m.id}`);
    try {
      await api.meals.remove(m.id);
      setMeals(ms => ms.filter(x => x.id !== m.id));
    } catch {
      setError(t('meals.error'));
    } finally {
      endPending(`meal-${m.id}`);
    }
  };

  const locale = i18n.resolvedLanguage || undefined;
  const byDay = mealsByDay(meals, days);
  const tonightMeals = tonight(meals, today);
  const open = list ? list.items.filter(i => !i.checked) : [];
  const done = list ? list.items.filter(i => i.checked) : [];

  let body: React.ReactNode = null;
  if (!loaded) {
    body = null;
  } else if (status && !status.enabled) {
    body = <p className={styles.empty}>{t('meals.notSetUp')}</p>;
  } else if (tab === 'tonight') {
    body = tonightMeals.length > 0 ? (
      <section className={styles.tonight} aria-label={t('meals.tabs.tonight')}>
        {tonightMeals.map(m => (
          <article key={m.id} className={styles.tonightCard}>
            <span className={styles.tonightWell}><Icon name="dish" /></span>
            <p className={styles.label}>{t(`meals.type.${m.meal}`)}</p>
            <h2 className={styles.tonightTitle}>{m.title}</h2>
            {m.total_time && <p className={styles.tonightMeta}><Icon name="clock" />{m.total_time}</p>}
            {m.recipe_slug && (
              <Button icon="chev" className={styles.tonightRecipe} onClick={() => navigate(recipePath(m.recipe_slug!))}>
                {t('meals.seeRecipe')}
              </Button>
            )}
          </article>
        ))}
      </section>
    ) : (
      <p className={styles.empty}>{t('meals.nothingTonight')}</p>
    );
  } else if (tab === 'week') {
    body = (
      <>
        {canEdit && <AddMeal days={days} onAdded={load} />}
        <div className={styles.week}>
          {days.map(d => {
            const dayMeals = byDay.get(d) ?? [];
            return (
              <section key={d} className={styles.day} aria-labelledby={`day-${d}`}>
                <h2 id={`day-${d}`} className={styles.label}>
                  {d === today ? t('meals.today') : parseDay(d).toLocaleDateString(locale, { weekday: 'long', day: 'numeric', month: 'short' })}
                </h2>
                {dayMeals.length === 0
                  ? <p className={styles.nothing}>{t('meals.nothingPlanned')}</p>
                  : dayMeals.map(m => (
                    <MealRow
                      key={m.id}
                      meal={m}
                      busy={pending.has(`meal-${m.id}`)}
                      onRemove={canEdit ? () => remove(m) : undefined}
                    />
                  ))}
              </section>
            );
          })}
        </div>
        {!canEdit && (
          <p className={styles.hint}>
            <Icon name="lock" />
            {t('meals.grownUpsHint')}{' '}
            <button type="button" className={styles.link} onClick={() => navigate('/login?next=/meals')}>
              {t('meals.signIn')}
            </button>
          </p>
        )}
      </>
    );
  } else if (tab === 'recipes') {
    body = <RecipesTab canEdit={canEdit} mealieUrl={status?.mealie_url} />;
  } else {
    body = list == null ? (
      <p className={styles.empty}>{t('meals.noList')}</p>
    ) : (
      <section className={styles.shopping} aria-label={list.name}>
        <h2 className={styles.label}>{list.name}</h2>
        {list.items.length === 0 && <p className={styles.nothing}>{t('meals.listEmpty')}</p>}
        {[...open, ...done].map(it => (
          <ChoreRow
            key={it.id}
            icon="check"
            title={it.label}
            state={it.checked ? 'done' : 'todo'}
            busy={pending.has(it.id)}
            onToggle={() => toggle(it.id, !it.checked)}
            checkLabel={it.checked ? t('meals.untick', { item: it.label }) : t('meals.tick', { item: it.label })}
          />
        ))}
      </section>
    );
  }

  return (
    <HouseScope persistent={persistent} className={styles.screen}>
      <div className={styles.page}>
        <header className={styles.top}>
          <button type="button" className={styles.back} onClick={back}>
            <Icon name="back" />
            {t('entry.picker.whosHere')}
          </button>
          <BrandMark wordmark={false} className={styles.mark} />
          <span />
        </header>

        <h1 className={styles.hello}>
          {t('meals.title')}{' '}
          {tonightMeals[0] && <span>{t('meals.tonightIs', { title: tonightMeals[0].title })}</span>}
        </h1>

        {error && <p className={styles.banner} role="alert">{error}</p>}

        <nav className={clsx('oc-tabs', 'oc-tabs--static', styles.tabs)} aria-label={t('meals.tabs.label')}>
          {(Object.keys(TAB_ICONS) as Tab[]).map(id => (
            <button
              key={id}
              type="button"
              className={clsx('oc-tab', tab === id && 'is-on')}
              aria-current={tab === id ? 'page' : undefined}
              onClick={() => setTab(id)}
            >
              <span className="oc-tab__pill">
                <Icon name={TAB_ICONS[id]} />
                <span className="oc-tab__label">{t(`meals.tabs.${id}`)}</span>
              </span>
            </button>
          ))}
        </nav>

        <main className={styles.body}>{body}</main>
      </div>
    </HouseScope>
  );
};

export default Meals;
