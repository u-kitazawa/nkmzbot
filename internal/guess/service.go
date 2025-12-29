package guess

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/geoscore"
)

var (
	ErrSessionNotFound      = errors.New("セッションが見つかりません")
	ErrSessionAlreadyExists = errors.New("このチャンネルには既にセッションが開始されています")
	ErrNoActiveSession      = errors.New("このチャンネルにはアクティブなセッションがありません")
	ErrAnswerNotSet         = errors.New("正解が設定されていません")
	ErrAlreadyGuessed       = errors.New("既に推測を送信しています")
)

// World map default: half Earth circumference
const DefaultMaxErrorDistance = 20015086.796

type Service struct {
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

type Session struct {
	ID                 int64
	ChannelID          string
	GuildID            int64
	OrganizerID        string
	Status             string
	AnswerLat          *float64
	AnswerLng          *float64
	AnswerURL          *string
	MaxErrorDistance   float64
	CreatedAt          time.Time
	ClosedAt           *time.Time
}

type Guess struct {
	ID              int64
	SessionID       int64
	UserID          string
	GuessLat        float64
	GuessLng        float64
	GuessURL        string
	Score           *int
	DistanceMeters  *float64
	CreatedAt       time.Time
}

type GuessResult struct {
	UserID         string
	Score          int
	DistanceMeters float64
}

// StartSession creates a new guess session in the channel.
func (s *Service) StartSession(ctx context.Context, channelID string, guildID int64, organizerID string) error {
	query := `
		INSERT INTO guess_sessions (channel_id, guild_id, organizer_id, status, max_error_distance)
		VALUES ($1, $2, $3, 'active', $4)
	`
	_, err := s.db.Pool().Exec(ctx, query, channelID, guildID, organizerID, DefaultMaxErrorDistance)
	if err != nil {
		// Check for unique constraint violation
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSessionAlreadyExists
		}
		return err
	}
	return nil
}

// StopSession ends the active session in the channel.
func (s *Service) StopSession(ctx context.Context, channelID string) error {
	query := `
		UPDATE guess_sessions
		SET status = 'closed', closed_at = CURRENT_TIMESTAMP
		WHERE channel_id = $1 AND status = 'active'
	`
	result, err := s.db.Pool().Exec(ctx, query, channelID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNoActiveSession
	}
	return nil
}

// GetActiveSession retrieves the active session for the channel.
func (s *Service) GetActiveSession(ctx context.Context, channelID string) (*Session, error) {
	query := `
		SELECT id, channel_id, guild_id, organizer_id, status, answer_lat, answer_lng, 
		       answer_url, max_error_distance, created_at, closed_at
		FROM guess_sessions
		WHERE channel_id = $1 AND status = 'active'
	`
	var sess Session
	err := s.db.Pool().QueryRow(ctx, query, channelID).Scan(
		&sess.ID, &sess.ChannelID, &sess.GuildID, &sess.OrganizerID, &sess.Status,
		&sess.AnswerLat, &sess.AnswerLng, &sess.AnswerURL, &sess.MaxErrorDistance,
		&sess.CreatedAt, &sess.ClosedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNoActiveSession
		}
		return nil, err
	}
	return &sess, nil
}

// AddGuess records a user's guess for the active session.
func (s *Service) AddGuess(ctx context.Context, channelID, userID string, guessLat, guessLng float64, guessURL string) error {
	sess, err := s.GetActiveSession(ctx, channelID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO guess_guesses (session_id, user_id, guess_lat, guess_lng, guess_url)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = s.db.Pool().Exec(ctx, query, sess.ID, userID, guessLat, guessLng, guessURL)
	if err != nil {
		// Check for unique constraint violation
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyGuessed
		}
		return err
	}
	return nil
}

// SetAnswer sets the correct answer for the active session and calculates scores.
func (s *Service) SetAnswer(ctx context.Context, channelID string, answerLat, answerLng float64, answerURL string) ([]GuessResult, error) {
	sess, err := s.GetActiveSession(ctx, channelID)
	if err != nil {
		return nil, err
	}

	// Update session with answer
	updateQuery := `
		UPDATE guess_sessions
		SET answer_lat = $1, answer_lng = $2, answer_url = $3
		WHERE id = $4
	`
	_, err = s.db.Pool().Exec(ctx, updateQuery, answerLat, answerLng, answerURL, sess.ID)
	if err != nil {
		return nil, err
	}

	// Get all guesses for this session
	guessQuery := `
		SELECT id, user_id, guess_lat, guess_lng
		FROM guess_guesses
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.Pool().Query(ctx, guessQuery, sess.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GuessResult
	for rows.Next() {
		var guessID int64
		var userID string
		var guessLat, guessLng float64
		if err := rows.Scan(&guessID, &userID, &guessLat, &guessLng); err != nil {
			return nil, err
		}

		// Calculate score and distance
		distance := geoscore.DistanceMeters(answerLat, answerLng, guessLat, guessLng)
		score := geoscore.GeoGuessrScore(answerLat, answerLng, guessLat, guessLng, sess.MaxErrorDistance)

		// Update guess with score and distance
		scoreUpdateQuery := `
			UPDATE guess_guesses
			SET score = $1, distance_meters = $2
			WHERE id = $3
		`
		_, err := s.db.Pool().Exec(ctx, scoreUpdateQuery, score, distance, guessID)
		if err != nil {
			return nil, err
		}

		results = append(results, GuessResult{
			UserID:         userID,
			Score:          score,
			DistanceMeters: distance,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// FormatDistance formats distance in a human-readable way.
func FormatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("%.0fm", meters)
	}
	return fmt.Sprintf("%.2fkm", meters/1000.0)
}
