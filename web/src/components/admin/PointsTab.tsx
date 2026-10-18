import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Pencil, Trash2 } from 'lucide-react';
import { api } from '../../api';
import type { User, PointBalance, Behaviour } from '../../types';
import { Avatar, Icon } from '../../design';
import { personColor } from './pickers';
import ui from './ui.module.css';
import styles from './PointsTab.module.css';

const signed = (n: number) => (n > 0 ? `+${n}` : `−${Math.abs(n)}`);

// The last behaviour applied, so the parent can see it landed and take it back.
interface Applied {
  userId: number;
  name: string;
  behaviour: Behaviour;
}

const BehaviourForm: React.FC<{
  initial?: Behaviour;
  onSave: () => void;
  onCancel: () => void;
}> = ({ initial, onSave, onCancel }) => {
  const { t } = useTranslation();
  const [name, setName] = useState(initial?.name ?? '');
  const [points, setPoints] = useState(initial ? String(initial.points) : '');
  const [saving, setSaving] = useState(false);
  const value = parseInt(points);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !value) return;
    setSaving(true);
    try {
      if (initial) {
        await api.behaviours.update(initial.id, { name: name.trim(), points: value, sort_order: initial.sort_order });
      } else {
        await api.behaviours.create({ name: name.trim(), points: value });
      }
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={clsx(ui.card, ui.formGrid)} onSubmit={handleSubmit}>
      <div className={ui.formRow}>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.pointsTab.behaviourName')}</span>
          <input
            className={ui.input}
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder={t('admin.pointsTab.behaviourNamePlaceholder')}
            autoFocus
          />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.pointsTab.amountLabel')}</span>
          <input
            className={ui.input}
            type="number"
            value={points}
            onChange={e => setPoints(e.target.value)}
            placeholder={t('admin.pointsTab.behaviourPointsPlaceholder')}
          />
        </label>
      </div>
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnGhost} onClick={onCancel}>{t('admin.pointsTab.cancelButton')}</button>
        <button type="submit" className={ui.btnPrimary} disabled={saving || !name.trim() || !value}>
          <Icon name="check" /> {initial ? t('admin.pointsTab.saveBehaviour') : t('admin.pointsTab.addBehaviour')}
        </button>
      </div>
    </form>
  );
};

export const PointsTab: React.FC = () => {
  const { t } = useTranslation();
  const [balances, setBalances] = useState<{ user: User; balance: number }[]>([]);
  const [behaviours, setBehaviours] = useState<Behaviour[]>([]);
  const [adjustUser, setAdjustUser] = useState<number | null>(null);
  const [adjustAmount, setAdjustAmount] = useState('');
  const [adjustNote, setAdjustNote] = useState('');
  const [saving, setSaving] = useState(false);
  const [applied, setApplied] = useState<Applied | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<number | null>(null);

  const load = useCallback(async () => {
    const [bals, usrs, behs] = await Promise.all([
      api.points.getAllBalances(),
      api.users.list(),
      api.behaviours.list(),
    ]);
    // Parents take part too, so everyone has a balance.
    setBalances(usrs.map(u => ({
      user: u,
      balance: bals.find((b: PointBalance) => b.user_id === u.id)?.balance || 0,
    })));
    setBehaviours(behs);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdjust = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!adjustUser || !adjustAmount) return;
    setSaving(true);
    try {
      // Every change is written to point_transactions by the API.
      await api.points.adjust(adjustUser, parseInt(adjustAmount), adjustNote || 'Admin adjustment');
      setAdjustUser(null);
      setAdjustAmount('');
      setAdjustNote('');
      load();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  const handleApply = async (user: User, behaviour: Behaviour) => {
    setSaving(true);
    try {
      await api.behaviours.apply(behaviour.id, user.id);
      setApplied({ userId: user.id, name: user.name, behaviour });
      load();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  // Undo is itself a ledger row, the same as undoing from the activity log.
  const handleUndoApplied = async () => {
    if (!applied) return;
    setSaving(true);
    try {
      await api.points.adjust(applied.userId, -applied.behaviour.points, `Undo: ${applied.behaviour.name}`);
      setApplied(null);
      load();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  const handleDelete = async (id: number) => {
    await api.behaviours.remove(id);
    load();
  };

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.pointsTab.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.pointsTab.subtitle')}</p>
        </div>
      </div>

      {applied && (
        <div className={clsx(ui.card, styles.applied)} role="status">
          <span>
            {t('admin.pointsTab.appliedMessage', {
              behaviour: applied.behaviour.name,
              points: signed(applied.behaviour.points),
              name: applied.name,
            })}
          </span>
          <button type="button" className={ui.btnGhost} onClick={handleUndoApplied} disabled={saving}>
            {t('admin.pointsTab.undoButton')}
          </button>
        </div>
      )}

      <div className={ui.grid}>
        {balances.map(({ user, balance }) => {
          const open = adjustUser === user.id;
          const formId = `adjust-${user.id}`;
          return (
            <div key={user.id} className={ui.card} data-person={personColor(user)}>
              <div className={styles.head}>
                <Avatar name={user.name} color={personColor(user)} size="md" />
                <span className={styles.name}>{user.name}</span>
                <span className={styles.balance}>
                  <Icon name="star" fill />
                  <span aria-hidden>{balance}</span>
                  <span className={ui.srOnlyText}>{t('design.points.label', { count: balance })}</span>
                </span>
              </div>

              {behaviours.length > 0 && (
                <div className={clsx(ui.chips, styles.behaviours)} role="group" aria-label={t('admin.pointsTab.behavioursFor', { name: user.name })}>
                  {behaviours.map(b => (
                    <button
                      key={b.id}
                      type="button"
                      className={clsx(ui.chip, b.points < 0 && styles.chipBad)}
                      disabled={saving}
                      onClick={() => handleApply(user, b)}
                    >
                      {b.name}
                      <span className={styles.chipPoints}>{signed(b.points)}</span>
                    </button>
                  ))}
                </div>
              )}

              <button
                type="button"
                className={ui.btnGhost}
                aria-expanded={open}
                aria-controls={formId}
                onClick={() => setAdjustUser(open ? null : user.id)}
              >
                {open ? t('admin.pointsTab.cancelButton') : t('admin.pointsTab.adjustButton')}
              </button>

              {open && (
                <form id={formId} className={styles.form} onSubmit={handleAdjust}>
                  <div className={ui.formRow}>
                    <label className={ui.field}>
                      <span className={ui.label}>{t('admin.pointsTab.amountLabel')}</span>
                      <input
                        className={ui.input}
                        type="number"
                        value={adjustAmount}
                        onChange={e => setAdjustAmount(e.target.value)}
                        placeholder={t('admin.pointsTab.amountPlaceholder')}
                        autoFocus
                      />
                    </label>
                    <label className={ui.field}>
                      <span className={ui.label}>{t('admin.pointsTab.reasonLabel')}</span>
                      <input
                        className={ui.input}
                        value={adjustNote}
                        onChange={e => setAdjustNote(e.target.value)}
                        placeholder={t('admin.pointsTab.reasonPlaceholder')}
                      />
                    </label>
                  </div>
                  <button type="submit" className={ui.btnPrimary} disabled={saving || !adjustAmount}>
                    <Icon name="check" /> {t('admin.pointsTab.applyButton')}
                  </button>
                </form>
              )}
            </div>
          );
        })}
      </div>

      <section className={ui.section}>
        <div className={ui.pageHead}>
          <div>
            <h2 className={ui.pageTitle}>{t('admin.pointsTab.behavioursTitle')}</h2>
            <p className={ui.pageSub}>{t('admin.pointsTab.behavioursSubtitle')}</p>
          </div>
          {!showForm && (
            <button type="button" className={ui.btnPrimary} onClick={() => { setEditing(null); setShowForm(true); }}>
              <Icon name="plus" /> {t('admin.pointsTab.addBehaviour')}
            </button>
          )}
        </div>

        {showForm && (
          <BehaviourForm
            onSave={() => { setShowForm(false); load(); }}
            onCancel={() => setShowForm(false)}
          />
        )}

        {behaviours.length === 0 ? (
          <p className={ui.emptyInline}>{t('admin.pointsTab.noBehaviours')}</p>
        ) : (
          <div className={ui.list}>
            {behaviours.map(b => editing === b.id ? (
              <BehaviourForm
                key={b.id}
                initial={b}
                onSave={() => { setEditing(null); load(); }}
                onCancel={() => setEditing(null)}
              />
            ) : (
              <div key={b.id} className={ui.row}>
                <span className={clsx(styles.points, b.points < 0 && styles.pointsBad)}>{signed(b.points)}</span>
                <div className={ui.rowMain}>
                  <span className={ui.rowTitle}>{b.name}</span>
                </div>
                <div className={ui.rowActions}>
                  <button
                    type="button"
                    className={ui.iconBtn}
                    aria-label={t('admin.pointsTab.editBehaviour', { name: b.name })}
                    title={t('admin.pointsTab.editBehaviour', { name: b.name })}
                    onClick={() => { setShowForm(false); setEditing(b.id); }}
                  >
                    <Pencil aria-hidden />
                  </button>
                  <button
                    type="button"
                    className={clsx(ui.iconBtn, ui.iconBtnDanger)}
                    aria-label={t('admin.pointsTab.deleteBehaviour', { name: b.name })}
                    title={t('admin.pointsTab.deleteBehaviour', { name: b.name })}
                    onClick={() => handleDelete(b.id)}
                  >
                    <Trash2 aria-hidden />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
};
