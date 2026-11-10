package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/service"
)

// memUsers is an in-memory fake UserRepository.
type memUsers struct {
	mu      sync.Mutex
	byID    map[string]model.User
	byEmail map[string]string
}

func newMemUsers() *memUsers {
	return &memUsers{byID: map[string]model.User{}, byEmail: map[string]string{}}
}

func (m *memUsers) Create(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[u.Email]; ok {
		return model.ErrEmailTaken
	}
	m.byID[u.ID] = *u
	m.byEmail[u.Email] = u.ID
	return nil
}

func (m *memUsers) GetByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byEmail[email]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	u := m.byID[id]
	return &u, nil
}

func (m *memUsers) GetByID(_ context.Context, id string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return &u, nil
}

const secret = "test-secret-test-secret-test-secret"

func newSvc() (*service.AuthService, *auth.TokenManager) {
	tm := auth.NewTokenManager(secret, "test-issuer", time.Hour)
	return service.NewAuthService(newMemUsers(), tm), tm
}

func TestRegisterThenLogin(t *testing.T) {
	svc, tm := newSvc()
	ctx := context.Background()

	u, err := svc.Register(ctx, "a@x.com", " Alice ", "password1")
	require.NoError(t, err)
	assert.Equal(t, "Alice", u.Name)
	assert.NotEqual(t, "password1", u.PasswordHash, "password must be hashed")

	res, err := svc.Login(ctx, "a@x.com", "password1")
	require.NoError(t, err)
	claims, err := tm.Parse(res.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, u.ID, claims.Subject)
	assert.Equal(t, "a@x.com", claims.Email)

	me, err := svc.Me(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Email, me.Email)
}

func TestEmailIsNormalised(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	u, err := svc.Register(ctx, "  A@X.com ", "A", "password1")
	require.NoError(t, err)
	assert.Equal(t, "a@x.com", u.Email)

	_, err = svc.Login(ctx, "a@x.com", "password1")
	assert.NoError(t, err)

	_, err = svc.Register(ctx, "a@X.COM", "dup", "password1")
	assert.ErrorIs(t, err, model.ErrEmailTaken)
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	_, err := svc.Register(ctx, "a@x.com", "A", "password1")
	require.NoError(t, err)

	_, wrongPassword := svc.Login(ctx, "a@x.com", "nope")
	_, unknownEmail := svc.Login(ctx, "nobody@x.com", "nope")
	assert.ErrorIs(t, wrongPassword, model.ErrInvalidCredentials)
	assert.ErrorIs(t, unknownEmail, model.ErrInvalidCredentials) // same error: no account enumeration
}
