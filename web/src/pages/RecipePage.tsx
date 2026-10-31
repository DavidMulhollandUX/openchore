import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api, APIError } from '../api';
import { useAuth } from '../AuthContext';
import type { Recipe, RecipeStep } from '../types';
import { HouseScope, Icon } from '../design';
import { BrandMark } from '../components/BrandMark/BrandMark';
import { recipeParagraphs } from './Meals.data';
import styles from './Meals.module.css';

/** Recipe text from Mealie, as paragraphs. No HTML is made from it. */
function RecipeText({ text }: { text: string }) {
  return (
    <>
      {recipeParagraphs(text).map((runs, i) => (
        <p key={i}>
          {runs.map((r, j) => r.strong ? <strong key={j}>{r.text}</strong> : r.em ? <em key={j}>{r.text}</em> : <React.Fragment key={j}>{r.text}</React.Fragment>)}
        </p>
      ))}
    </>
  );
}

function Notes({ notes }: { notes: RecipeStep[] }) {
  const { t } = useTranslation();
  if (notes.length === 0) return null;
  return (
    <section className={styles.recipeSection} aria-labelledby="recipe-notes">
      <h2 id="recipe-notes" className={styles.label}>{t('meals.recipe.notes')}</h2>
      {notes.map((n, i) => (
        <div key={i} className={styles.recipeNote}>
          {n.title && <h3 className={styles.recipeStepTitle}>{n.title}</h3>}
          <RecipeText text={n.text} />
        </div>
      ))}
    </section>
  );
}

/** One recipe from Mealie (/meals/recipes/:slug), to cook from at the
 *  tablet. Ingredients tick off as they go in; nothing is saved. */
export const RecipePage: React.FC = () => {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { session } = useAuth();
  const [recipe, setRecipe] = useState<Recipe | null>(null);
  const [error, setError] = useState('');
  const [used, setUsed] = useState<ReadonlySet<number>>(new Set());

  useEffect(() => {
    let alive = true;
    setRecipe(null);
    setUsed(new Set());
    api.meals.recipe(slug)
      .then(r => { if (alive) { setRecipe(r); setError(''); } })
      .catch(err => {
        if (!alive) return;
        const missing = err instanceof APIError && err.status === 404;
        setError(missing ? t('meals.recipe.missing') : t('meals.error'));
      });
    return () => { alive = false; };
  }, [slug, t]);

  // Back to wherever the recipe was opened from (keeping its tab), or Meals.
  const back = () => {
    const idx = (window.history.state as { idx?: number } | null)?.idx ?? 0;
    if (idx > 0) navigate(-1);
    else navigate('/meals?tab=recipes');
  };

  const toggle = (i: number) => setUsed(u => {
    const n = new Set(u);
    if (n.has(i)) n.delete(i); else n.add(i);
    return n;
  });

  const facts = recipe ? [
    { label: t('meals.recipe.prep'), value: recipe.prep_time },
    { label: t('meals.recipe.cook'), value: recipe.cook_time },
    { label: t('meals.recipe.total'), value: recipe.total_time },
    { label: t('meals.recipe.serves'), value: recipe.servings || recipe.yield },
  ].filter(f => f.value) : [];

  return (
    <HouseScope persistent={session?.persistent ?? false} className={styles.screen}>
      <div className={styles.page}>
        <header className={styles.top}>
          <button type="button" className={styles.back} onClick={back}>
            <Icon name="back" />
            {t('meals.backToMeals')}
          </button>
          <BrandMark wordmark={false} className={styles.mark} />
          <span />
        </header>

        {error && <p className={clsx(styles.empty, styles.recipeError)} role="alert">{error}</p>}

        {recipe && (
          <article className={styles.recipe}>
            {recipe.image && <img className={styles.recipeImage} src={recipe.image} alt="" />}
            <h1 className={styles.recipeTitle}>{recipe.name}</h1>
            {recipe.description && <p className={styles.recipeLede}>{recipe.description}</p>}
            {facts.length > 0 && (
              <dl className={styles.facts}>
                {facts.map(f => (
                  <div key={f.label} className={styles.fact}>
                    <dt>{f.label}</dt>
                    <dd>{f.value}</dd>
                  </div>
                ))}
              </dl>
            )}

            <div className={styles.recipeCols}>
              {recipe.ingredients.length > 0 && (
                <section className={styles.recipeSection} aria-labelledby="recipe-ingredients">
                  <h2 id="recipe-ingredients" className={styles.label}>{t('meals.recipe.ingredients')}</h2>
                  <ul className={styles.ingredients}>
                    {recipe.ingredients.map((ing, i) => ing.section ? (
                      <li key={i} className={styles.ingredientSection}>{ing.section}</li>
                    ) : (
                      <li key={i}>
                        <button
                          type="button"
                          className={clsx(styles.ingredient, used.has(i) && styles.ingredientUsed)}
                          aria-pressed={used.has(i)}
                          onClick={() => toggle(i)}
                        >
                          <span className={styles.ingredientBox} aria-hidden="true">{used.has(i) && <Icon name="check" />}</span>
                          {ing.text}
                        </button>
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              <div>
                {recipe.steps.length > 0 && (
                  <section className={styles.recipeSection} aria-labelledby="recipe-steps">
                    <h2 id="recipe-steps" className={styles.label}>{t('meals.recipe.method')}</h2>
                    <ol className={styles.steps}>
                      {recipe.steps.map((st, i) => (
                        <li key={i} className={styles.step}>
                          <span className={styles.stepNumber} aria-hidden="true">{i + 1}</span>
                          <div className={styles.stepText}>
                            {st.title && <h3 className={styles.recipeStepTitle}>{st.title}</h3>}
                            <RecipeText text={st.text} />
                          </div>
                        </li>
                      ))}
                    </ol>
                  </section>
                )}
                <Notes notes={recipe.notes} />
              </div>
            </div>
          </article>
        )}
      </div>
    </HouseScope>
  );
};

export default RecipePage;
