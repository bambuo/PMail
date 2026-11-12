package mfa

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/models"
)

type mfaChallenge struct {
	userID   int
	expires  time.Time
	attempts int
}

var (
	mfaChallengeMu sync.Mutex
	mfaChallenges  = map[string]*mfaChallenge{}
)

func (s *Service) CreateChallenge(userID int) (string, error) {
	var user models.User
	ok, err := s.engine.ID(userID).Get(&user)
	if err != nil {
		return "", err
	}
	if !ok || user.ID == 0 || user.MfaEnabled != 1 {
		return "", mfadto.ErrNotEnabled
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)
	now := time.Now()
	mfaChallengeMu.Lock()
	for key, challenge := range mfaChallenges {
		if now.After(challenge.expires) {
			delete(mfaChallenges, key)
		}
	}
	mfaChallenges[token] = &mfaChallenge{userID: userID, expires: now.Add(mfaChallengeTTL)}
	mfaChallengeMu.Unlock()
	return token, nil
}

func (s *Service) VerifyChallenge(token, code string) (*models.User, error) {
	if token == "" || code == "" {
		return nil, mfadto.ErrInvalidChallenge
	}
	mfaChallengeMu.Lock()
	challenge, ok := mfaChallenges[token]
	if !ok {
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrInvalidChallenge
	}
	if time.Now().After(challenge.expires) {
		delete(mfaChallenges, token)
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrInvalidChallenge
	}
	if challenge.attempts >= mfaChallengeMaxTries {
		delete(mfaChallenges, token)
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrTooManyAttempts
	}
	challenge.attempts++
	userID := challenge.userID
	mfaChallengeMu.Unlock()

	var user models.User
	ok, err := s.engine.ID(userID).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 || user.Disabled != 0 || user.MfaEnabled != 1 {
		return nil, mfadto.ErrInvalidChallenge
	}
	valid, err := s.verifyCode(&user, code, true)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, mfadto.ErrInvalidCode
	}
	mfaChallengeMu.Lock()
	delete(mfaChallenges, token)
	mfaChallengeMu.Unlock()
	return &user, nil
}
