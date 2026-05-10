// Package session gerencia sessões em Redis com TTL idle-refreshed.
//
// Token: 32 bytes aleatórios em base64url sem padding (43 chars).
// Chave Redis: "session:<token>".
// Valor: JSON do struct Session.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound indica sessão inexistente ou expirada.
var ErrNotFound = errors.New("sessão não encontrada")

const keyPrefix = "session:"

// Session é o payload guardado no Redis.
type Session struct {
	Token      string    `json:"token"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	IP         string    `json:"ip"`
}

// Store abstrai operações de sessão sobre Redis.
type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

// New conecta a partir de uma URL no formato redis://... e devolve um Store
// com TTL idle (ex.: 30 minutos).
func New(redisURL string, ttl time.Duration) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Store{rdb: rdb, ttl: ttl}, nil
}

func (s *Store) TTL() time.Duration { return s.ttl }

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create gera um novo token, persiste a sessão e devolve o objeto.
func (s *Store) Create(ctx context.Context, userID, ip string) (*Session, error) {
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &Session{
		Token: tok, UserID: userID,
		CreatedAt: now, LastSeenAt: now, IP: ip,
	}
	if err := s.write(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// Get devolve a sessão sem renovar o TTL.
func (s *Store) Get(ctx context.Context, token string) (*Session, error) {
	v, err := s.rdb.Get(ctx, keyPrefix+token).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(v, &sess); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &sess, nil
}

// Touch renova o TTL e atualiza LastSeenAt.
func (s *Store) Touch(ctx context.Context, sess *Session) error {
	sess.LastSeenAt = time.Now().UTC()
	return s.write(ctx, sess)
}

// Delete invalida uma sessão pelo token.
func (s *Store) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, keyPrefix+token).Err()
}

// DeleteAllForUser remove todas as sessões de um usuário (logout global).
func (s *Store) DeleteAllForUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	iter := s.rdb.Scan(ctx, 0, keyPrefix+"*", 200).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		v, err := s.rdb.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var sess Session
		if json.Unmarshal(v, &sess) != nil {
			continue
		}
		if sess.UserID == userID {
			if err := s.rdb.Del(ctx, key).Err(); err == nil {
				count++
			}
		}
	}
	return count, iter.Err()
}

func (s *Store) write(ctx context.Context, sess *Session) error {
	b, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefix+sess.Token, b, s.ttl).Err()
}
