// Package store provides SQLite persistence for StableGuard.
//
// Tables:
//   - price_snapshots  — Pyth price feed records (every pipeline tick)
//   - ai_decisions     — AI agent decisions with rationale
//   - rebalance_history — executed on-chain rebalances
//   - risk_events       — risk level crossings (> threshold)
package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB is the StableGuard persistent store.
type DB struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("store open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	s := &DB{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("store migrate: %w", err)
	}
	log.Printf("[store] SQLite opened at %s", path)
	return s, nil
}

// Close closes the database.
func (s *DB) Close() error { return s.db.Close() }

// ── Schema ─────────────────────────────────────────────────────────────────

func (s *DB) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS price_snapshots (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ts          INTEGER NOT NULL,
		symbol      TEXT    NOT NULL,
		price       REAL    NOT NULL,
		confidence  REAL    NOT NULL,
		feed_id     TEXT    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_ps_ts     ON price_snapshots(ts);
	CREATE INDEX IF NOT EXISTS idx_ps_symbol ON price_snapshots(symbol, ts);

	CREATE TABLE IF NOT EXISTS ai_decisions (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		ts                 INTEGER NOT NULL,
		action             TEXT    NOT NULL,
		from_index         INTEGER NOT NULL,
		to_index           INTEGER NOT NULL,
		suggested_fraction REAL    NOT NULL,
		confidence         INTEGER NOT NULL,
		rationale          TEXT    NOT NULL,
		risk_analysis      TEXT    NOT NULL,
		yield_analysis     TEXT    NOT NULL,
		risk_level         REAL    NOT NULL,
		exec_sig           TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_ad_ts ON ai_decisions(ts);

	CREATE TABLE IF NOT EXISTS rebalance_history (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		ts         INTEGER NOT NULL,
		from_index INTEGER NOT NULL,
		to_index   INTEGER NOT NULL,
		amount     INTEGER NOT NULL,
		signature  TEXT    NOT NULL,
		risk_level REAL    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_rh_ts ON rebalance_history(ts);

	CREATE TABLE IF NOT EXISTS risk_events (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		ts         INTEGER NOT NULL,
		risk_level REAL    NOT NULL,
		deviation  REAL    NOT NULL,
		summary    TEXT    NOT NULL,
		action     TEXT    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_re_ts ON risk_events(ts);

	CREATE TABLE IF NOT EXISTS yield_positions (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		protocol     TEXT    NOT NULL,
		token        TEXT    NOT NULL,
		amount       REAL    NOT NULL,
		entry_apy    REAL    NOT NULL,
		deposited_at INTEGER NOT NULL,
		withdrawn_at INTEGER,
		earned       REAL    NOT NULL DEFAULT 0,
		deposit_sig  TEXT    NOT NULL DEFAULT '',
		withdraw_sig TEXT    NOT NULL DEFAULT '',
		is_active    INTEGER NOT NULL DEFAULT 1
	);
	CREATE INDEX IF NOT EXISTS idx_yp_active ON yield_positions(is_active);
	`)
	return err
}

// ── Yield positions ────────────────────────────────────────────────────────

// YieldPosition is one row in yield_positions.
type YieldPosition struct {
	ID          int64
	Protocol    string
	Token       string
	Amount      float64
	EntryAPY    float64
	DepositedAt time.Time
	WithdrawnAt *time.Time
	Earned      float64
	DepositSig  string
	WithdrawSig string
	IsActive    bool
}

// SaveYieldPosition inserts a new open position.
func (s *DB) SaveYieldPosition(protocol, token string, amount, entryAPY float64, depositSig string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO yield_positions (protocol, token, amount, entry_apy, deposited_at, deposit_sig)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		protocol, token, amount, entryAPY, time.Now().Unix(), depositSig,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CloseYieldPosition marks a position as withdrawn and records earned yield.
func (s *DB) CloseYieldPosition(id int64, earned float64, withdrawSig string) error {
	_, err := s.db.Exec(
		`UPDATE yield_positions SET is_active=0, withdrawn_at=?, earned=?, withdraw_sig=? WHERE id=?`,
		time.Now().Unix(), earned, withdrawSig, id,
	)
	return err
}

// ActiveYieldPosition returns the currently open position, if any.
func (s *DB) ActiveYieldPosition() (*YieldPosition, error) {
	row := s.db.QueryRow(
		`SELECT id, protocol, token, amount, entry_apy, deposited_at, earned, deposit_sig
		 FROM yield_positions WHERE is_active=1 ORDER BY deposited_at DESC LIMIT 1`,
	)
	var p YieldPosition
	var depositedAt int64
	err := row.Scan(&p.ID, &p.Protocol, &p.Token, &p.Amount, &p.EntryAPY,
		&depositedAt, &p.Earned, &p.DepositSig)
	if err != nil {
		return nil, err // sql.ErrNoRows if none
	}
	p.DepositedAt = time.Unix(depositedAt, 0)
	p.IsActive = true
	return &p, nil
}

// RecentYieldPositions returns the last N positions.
func (s *DB) RecentYieldPositions(limit int) ([]YieldPosition, error) {
	rows, err := s.db.Query(
		`SELECT id, protocol, token, amount, entry_apy, deposited_at,
		        earned, deposit_sig, withdraw_sig, is_active
		 FROM yield_positions ORDER BY deposited_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []YieldPosition
	for rows.Next() {
		var p YieldPosition
		var depositedAt int64
		var active int
		if err := rows.Scan(&p.ID, &p.Protocol, &p.Token, &p.Amount, &p.EntryAPY,
			&depositedAt, &p.Earned, &p.DepositSig, &p.WithdrawSig, &active); err != nil {
			continue
		}
		p.DepositedAt = time.Unix(depositedAt, 0)
		p.IsActive = active == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Price snapshots ────────────────────────────────────────────────────────

// PriceRow is one row in price_snapshots.
type PriceRow struct {
	ID         int64
	Ts         time.Time
	Symbol     string
	Price      float64
	Confidence float64
	FeedID     string
}

// SavePrice inserts one price record.
func (s *DB) SavePrice(symbol, feedID string, price, confidence float64) error {
	_, err := s.db.Exec(
		`INSERT INTO price_snapshots(ts,symbol,price,confidence,feed_id) VALUES(?,?,?,?,?)`,
		time.Now().Unix(), symbol, price, confidence, feedID,
	)
	return err
}

// RecentPrices returns the last `limit` rows for a symbol.
func (s *DB) RecentPrices(symbol string, limit int) ([]PriceRow, error) {
	rows, err := s.db.Query(
		`SELECT id,ts,symbol,price,confidence,feed_id
		 FROM price_snapshots WHERE symbol=?
		 ORDER BY ts DESC LIMIT ?`, symbol, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrices(rows)
}

// PricesSince returns all rows for a symbol after `since`.
func (s *DB) PricesSince(symbol string, since time.Time) ([]PriceRow, error) {
	rows, err := s.db.Query(
		`SELECT id,ts,symbol,price,confidence,feed_id
		 FROM price_snapshots WHERE symbol=? AND ts>=?
		 ORDER BY ts ASC`, symbol, since.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrices(rows)
}

func scanPrices(rows *sql.Rows) ([]PriceRow, error) {
	var out []PriceRow
	for rows.Next() {
		var r PriceRow
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.Symbol, &r.Price, &r.Confidence, &r.FeedID); err != nil {
			return nil, err
		}
		r.Ts = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── AI decisions ───────────────────────────────────────────────────────────

// DecisionRow is one row in ai_decisions.
type DecisionRow struct {
	ID                int64
	Ts                time.Time
	Action            string
	FromIndex         int
	ToIndex           int
	SuggestedFraction float64
	Confidence        int
	Rationale         string
	RiskAnalysis      string
	YieldAnalysis     string
	RiskLevel         float64
	ExecSig           string
}

// SaveDecision inserts an AI decision.
func (s *DB) SaveDecision(d DecisionRow) error {
	_, err := s.db.Exec(`
		INSERT INTO ai_decisions
		(ts,action,from_index,to_index,suggested_fraction,confidence,rationale,risk_analysis,yield_analysis,risk_level,exec_sig)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		time.Now().Unix(),
		d.Action, d.FromIndex, d.ToIndex, d.SuggestedFraction,
		d.Confidence, d.Rationale, d.RiskAnalysis, d.YieldAnalysis,
		d.RiskLevel, d.ExecSig,
	)
	return err
}

// RecentDecisions returns the last `limit` AI decisions.
func (s *DB) RecentDecisions(limit int) ([]DecisionRow, error) {
	rows, err := s.db.Query(`
		SELECT id,ts,action,from_index,to_index,suggested_fraction,confidence,
		       rationale,risk_analysis,yield_analysis,risk_level,exec_sig
		FROM ai_decisions ORDER BY ts DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DecisionRow
	for rows.Next() {
		var r DecisionRow
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.Action, &r.FromIndex, &r.ToIndex,
			&r.SuggestedFraction, &r.Confidence, &r.Rationale,
			&r.RiskAnalysis, &r.YieldAnalysis, &r.RiskLevel, &r.ExecSig); err != nil {
			return nil, err
		}
		r.Ts = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Rebalance history ──────────────────────────────────────────────────────

// RebalanceRow is one row in rebalance_history.
type RebalanceRow struct {
	ID        int64
	Ts        time.Time
	FromIndex int
	ToIndex   int
	Amount    uint64
	Signature string
	RiskLevel float64
}

// SaveRebalance inserts a completed rebalance.
func (s *DB) SaveRebalance(fromIndex, toIndex int, amount uint64, sig string, riskLevel float64) error {
	_, err := s.db.Exec(`
		INSERT INTO rebalance_history(ts,from_index,to_index,amount,signature,risk_level)
		VALUES(?,?,?,?,?,?)`,
		time.Now().Unix(), fromIndex, toIndex, amount, sig, riskLevel,
	)
	return err
}

// RecentRebalances returns the last `limit` rebalances.
func (s *DB) RecentRebalances(limit int) ([]RebalanceRow, error) {
	rows, err := s.db.Query(`
		SELECT id,ts,from_index,to_index,amount,signature,risk_level
		FROM rebalance_history ORDER BY ts DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RebalanceRow
	for rows.Next() {
		var r RebalanceRow
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.FromIndex, &r.ToIndex, &r.Amount, &r.Signature, &r.RiskLevel); err != nil {
			return nil, err
		}
		r.Ts = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Risk events ────────────────────────────────────────────────────────────

// RiskEventRow is one row in risk_events.
type RiskEventRow struct {
	ID        int64
	Ts        time.Time
	RiskLevel float64
	Deviation float64
	Summary   string
	Action    string
}

// SaveRiskEvent inserts a risk event (called when risk crosses a threshold).
func (s *DB) SaveRiskEvent(riskLevel, deviation float64, summary, action string) error {
	_, err := s.db.Exec(`
		INSERT INTO risk_events(ts,risk_level,deviation,summary,action)
		VALUES(?,?,?,?,?)`,
		time.Now().Unix(), riskLevel, deviation, summary, action,
	)
	return err
}

// RecentRiskEvents returns the last `limit` risk events.
func (s *DB) RecentRiskEvents(limit int) ([]RiskEventRow, error) {
	rows, err := s.db.Query(`
		SELECT id,ts,risk_level,deviation,summary,action
		FROM risk_events ORDER BY ts DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RiskEventRow
	for rows.Next() {
		var r RiskEventRow
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.RiskLevel, &r.Deviation, &r.Summary, &r.Action); err != nil {
			return nil, err
		}
		r.Ts = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Stats ──────────────────────────────────────────────────────────────────

// Stats holds aggregate statistics.
type Stats struct {
	TotalDecisions  int
	TotalRebalances int
	TotalRiskEvents int
	AvgRiskLevel    float64
	LastDecisionTs  *time.Time
}

// GetStats returns aggregate counts.
func (s *DB) GetStats() (Stats, error) {
	var st Stats
	s.db.QueryRow(`SELECT COUNT(*) FROM ai_decisions`).Scan(&st.TotalDecisions)
	s.db.QueryRow(`SELECT COUNT(*) FROM rebalance_history`).Scan(&st.TotalRebalances)
	s.db.QueryRow(`SELECT COUNT(*) FROM risk_events`).Scan(&st.TotalRiskEvents)
	s.db.QueryRow(`SELECT AVG(risk_level) FROM risk_events`).Scan(&st.AvgRiskLevel)

	var lastTs int64
	if err := s.db.QueryRow(`SELECT MAX(ts) FROM ai_decisions`).Scan(&lastTs); err == nil && lastTs > 0 {
		t := time.Unix(lastTs, 0)
		st.LastDecisionTs = &t
	}
	return st, nil
}
