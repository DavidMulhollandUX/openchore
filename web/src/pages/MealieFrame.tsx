import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import { HouseScope, Icon } from '../design';
import { BrandMark } from '../components/BrandMark/BrandMark';
import styles from './Meals.module.css';

/** Mealie inside OpenChore's frame (/meals/mealie), so recipes don't open
 *  a separate window. Mealie runs on its own port, which is the same site,
 *  so its sign-in cookie still works in the frame. */
export const MealieFrame: React.FC = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { session } = useAuth();
  const [url, setUrl] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    api.meals.status()
      .then(s => {
        if (!alive) return;
        if (s.enabled && s.mealie_url) setUrl(s.mealie_url);
        else navigate('/meals', { replace: true });
      })
      .catch(() => { if (alive) navigate('/meals', { replace: true }); });
    return () => { alive = false; };
  }, [navigate]);

  return (
    <HouseScope persistent={session?.persistent ?? false} className={styles.frameScreen}>
      <header className={styles.frameBar}>
        <button type="button" className={styles.back} onClick={() => navigate('/meals')}>
          <Icon name="back" />
          {t('meals.backToMeals')}
        </button>
        <BrandMark wordmark={false} className={styles.mark} />
        <span />
      </header>
      {url && <iframe className={styles.frame} src={url} title="Mealie" allow="clipboard-write; fullscreen" />}
    </HouseScope>
  );
};
