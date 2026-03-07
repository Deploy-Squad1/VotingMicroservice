package storage

import (
	"database/sql"
	"time"
	"votingmicroservice/internal/models"
)

type Storage struct {
	db *sql.DB
}

func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func (s *Storage) HasActivePoll(targetUserID int, pollType string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM polls 
			WHERE target_user_id = $1 
			  AND poll_type = $2 
			  AND status = 'active'
			  AND expires_at > NOW()
		)
	`
	err := s.db.QueryRow(query, targetUserID, pollType).Scan(&exists)
	return exists, err
}

func (s *Storage) CreatePoll(pollType string, targetUserID, initiatorID int) (int, error) {
	var pollID int
	query := `
		INSERT INTO polls (poll_type, target_user_id, initiator_id, expires_at)
		VALUES ($1, $2, $3, NOW() + INTERVAL '24 hours')
		RETURNING id
	`
	err := s.db.QueryRow(query, pollType, targetUserID, initiatorID).Scan(&pollID)
	return pollID, err
}

func (s *Storage) GetUserGroup(userID int) (int, error) {
	var groupID int
	query := `SELECT group_id FROM app_user_groups WHERE user_id = $1 LIMIT 1`
	err := s.db.QueryRow(query, userID).Scan(&groupID)
	if err != nil {
		return 0, err
	}
	return groupID, nil
}

func (s *Storage) GetUserDateJoined(userID int) (time.Time, error) {
	var dateJoined time.Time
	query := `SELECT date_joined FROM app_user WHERE id = $1`
	err := s.db.QueryRow(query, userID).Scan(&dateJoined)
	return dateJoined, err
}

func (s *Storage) IsUserInquisitor(userID int) (bool, error) {
	var isInquisitor bool
	query := `SELECT is_inquisitor FROM app_user WHERE id = $1`
	err := s.db.QueryRow(query, userID).Scan(&isInquisitor)
	return isInquisitor, err
}

func (s *Storage) AddPollVote(pollID, voterID int, vote bool) error {
	query := `INSERT INTO polls_votes (poll_id, voter_id, vote) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, pollID, voterID, vote)
	return err
}

func (s *Storage) GetPoll(pollID int) (models.Poll, error) {
	var p models.Poll
	query := `SELECT id, poll_type, target_user_id, initiator_id, status, created_at, expires_at FROM polls WHERE id = $1`
	err := s.db.QueryRow(query, pollID).Scan(&p.ID, &p.PollType, &p.TargetUserID, &p.InitiatorID, &p.Status, &p.CreatedAt, &p.ExpiresAt)
	return p, err
}

func (s *Storage) GetExpiredActivePolls() ([]models.Poll, error) {
	query := `SELECT id, poll_type, target_user_id, initiator_id, status, created_at, expires_at FROM polls WHERE status = 'active' AND expires_at <= NOW()`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var polls []models.Poll
	for rows.Next() {
		var p models.Poll
		if err := rows.Scan(&p.ID, &p.PollType, &p.TargetUserID, &p.InitiatorID, &p.Status, &p.CreatedAt, &p.ExpiresAt); err == nil {
			polls = append(polls, p)
		}
	}
	return polls, nil
}

func (s *Storage) CountVotes(pollID int) (int, int, error) {
	var votesFor, votesAgainst int
	errFor := s.db.QueryRow(`SELECT COUNT(*) FROM polls_votes WHERE poll_id = $1 AND vote = true`, pollID).Scan(&votesFor)
	errAgainst := s.db.QueryRow(`SELECT COUNT(*) FROM polls_votes WHERE poll_id = $1 AND vote = false`, pollID).Scan(&votesAgainst)

	if errFor != nil || errAgainst != nil {
		return 0, 0, errFor
	}
	return votesFor, votesAgainst, nil
}

func (s *Storage) UpdatePollStatus(pollID int, status string) error {
	_, err := s.db.Exec(`UPDATE polls SET status = $1 WHERE id = $2`, status, pollID)
	return err
}

func (s *Storage) ExecuteKick(targetUserID int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM app_user_groups WHERE user_id = $1`, targetUserID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DELETE FROM map_artifacts WHERE created_by = $1`, targetUserID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DELETE FROM app_user WHERE id = $1`, targetUserID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (s *Storage) ExecuteUpgrade(targetUserID int) error {
	_, err := s.db.Exec(`UPDATE app_user_groups SET group_id = group_id + 1 WHERE user_id = $1`, targetUserID)
	return err
}
