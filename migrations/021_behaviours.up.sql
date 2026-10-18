-- Behaviours: a parent-managed list of named behaviours, each worth a fixed
-- number of points (positive for good behaviour, negative for misbehaviour).
-- Applying one to a kid writes a point_transactions row with reason
-- 'behaviour', reference_id = behaviours.id and the behaviour's name as the
-- note, so the ledger still reads correctly after a behaviour is renamed or
-- deleted.
CREATE TABLE behaviours (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    points INTEGER NOT NULL CHECK (points != 0),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Widen the point_transactions.reason CHECK to accept 'behaviour'.
-- SQLite doesn't support ALTER CHECK in place, so we rebuild the table.
PRAGMA foreign_keys = OFF;

CREATE TABLE point_transactions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN (
        'chore_complete', 'chore_uncomplete', 'reward_redeem',
        'streak_bonus', 'admin_adjust', 'expiry_penalty',
        'points_decay', 'missed_chore',
        'commit_to_goal', 'goal_break',
        'behaviour'
    )),
    reference_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO point_transactions_new
    (id, user_id, amount, reason, reference_id, note, idempotency_key, created_at)
SELECT
    id, user_id, amount, reason, reference_id, note, idempotency_key, created_at
FROM point_transactions;

DROP TABLE point_transactions;
ALTER TABLE point_transactions_new RENAME TO point_transactions;

CREATE INDEX idx_point_tx_user ON point_transactions(user_id);
CREATE INDEX idx_point_tx_ref ON point_transactions(reason, reference_id);
CREATE UNIQUE INDEX idx_point_tx_idempotency_key
    ON point_transactions(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

PRAGMA foreign_keys = ON;
