package auth_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
)

// errRepository はrepositoryの技術的失敗を模したerror。
var errRepository = errors.New("repository failure")

// fakeUserRepository はmemory上でUserRepositoryを実装する。
type fakeUserRepository struct {
	mutex          sync.Mutex
	usersByEmail   map[string]domainauth.User
	hashesByUserID map[domainauth.UserID]domainauth.PasswordHash
	nextID         int64

	failOnExists   bool
	failOnFindUser bool
	failOnCreate   bool
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		usersByEmail:   make(map[string]domainauth.User),
		hashesByUserID: make(map[domainauth.UserID]domainauth.PasswordHash),
		nextID:         1,
	}
}

func (r *fakeUserRepository) ExistsActiveByEmail(
	_ context.Context,
	email domainauth.Email,
) (bool, error) {
	if r.failOnExists {
		return false, errRepository
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	_, ok := r.usersByEmail[email.String()]
	return ok, nil
}

func (r *fakeUserRepository) FindActiveByEmail(
	_ context.Context,
	email domainauth.Email,
) (domainauth.User, error) {
	if r.failOnFindUser {
		return domainauth.User{}, errRepository
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	user, ok := r.usersByEmail[email.String()]
	if !ok {
		return domainauth.User{}, domainauth.ErrUserNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) FindActiveByID(
	_ context.Context,
	id domainauth.UserID,
) (domainauth.User, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, user := range r.usersByEmail {
		if user.ID() == id {
			return user, nil
		}
	}
	return domainauth.User{}, domainauth.ErrUserNotFound
}

func (r *fakeUserRepository) Create(
	_ context.Context,
	user domainauth.User,
	passwordHash domainauth.PasswordHash,
) (domainauth.User, error) {
	if r.failOnCreate {
		return domainauth.User{}, errRepository
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.usersByEmail[user.Email().String()]; exists {
		return domainauth.User{}, domainauth.ErrEmailAlreadyRegistered
	}

	persisted := user.WithID(domainauth.UserID(r.nextID))
	r.nextID++

	r.usersByEmail[persisted.Email().String()] = persisted
	r.hashesByUserID[persisted.ID()] = passwordHash

	return persisted, nil
}

func (r *fakeUserRepository) FindPasswordHashByUserID(
	_ context.Context,
	id domainauth.UserID,
) (domainauth.PasswordHash, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	hash, ok := r.hashesByUserID[id]
	if !ok {
		return domainauth.PasswordHash{}, domainauth.ErrPasswordAuthNotFound
	}
	return hash, nil
}

var _ domainauth.UserRepository = (*fakeUserRepository)(nil)

// fakeAuthSessionRepository はmemory上でAuthSessionRepositoryを実装する。
type fakeAuthSessionRepository struct {
	mutex    sync.Mutex
	sessions map[string]domainauth.AuthSession
	users    map[domainauth.UserID]domainauth.User

	failOnCreate  bool
	failOnRefresh bool
	refreshCount  int
}

func newFakeAuthSessionRepository() *fakeAuthSessionRepository {
	return &fakeAuthSessionRepository{
		sessions: make(map[string]domainauth.AuthSession),
		users:    make(map[domainauth.UserID]domainauth.User),
	}
}

func hashKey(tokenHash domainauth.SessionTokenHash) string {
	return string(tokenHash.Bytes())
}

func (r *fakeAuthSessionRepository) Create(
	_ context.Context,
	session domainauth.AuthSession,
) (domainauth.AuthSession, error) {
	if r.failOnCreate {
		return domainauth.AuthSession{}, errRepository
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()

	persisted := session.WithID(int64(len(r.sessions) + 1))
	r.sessions[hashKey(session.TokenHash())] = persisted
	return persisted, nil
}

func (r *fakeAuthSessionRepository) FindLiveWithUserByTokenHash(
	_ context.Context,
	tokenHash domainauth.SessionTokenHash,
	evaluatedAt time.Time,
) (domainauth.AuthSession, domainauth.User, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, ok := r.sessions[hashKey(tokenHash)]
	if !ok || !session.IsLive(evaluatedAt) {
		return domainauth.AuthSession{}, domainauth.User{}, domainauth.ErrAuthSessionNotFound
	}

	user, ok := r.users[session.UserID()]
	if !ok {
		return domainauth.AuthSession{}, domainauth.User{}, domainauth.ErrAuthSessionNotFound
	}
	return session, user, nil
}

func (r *fakeAuthSessionRepository) RefreshLastUsedAt(
	_ context.Context,
	tokenHash domainauth.SessionTokenHash,
	usedAt time.Time,
) error {
	if r.failOnRefresh {
		return errRepository
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.refreshCount++

	session, ok := r.sessions[hashKey(tokenHash)]
	if !ok {
		return nil
	}
	r.sessions[hashKey(tokenHash)] = domainauth.ReconstructAuthSession(
		domainauth.ReconstructAuthSessionParams{
			ID:         session.ID(),
			PublicID:   session.PublicID(),
			UserID:     session.UserID(),
			TokenHash:  session.TokenHash(),
			IssuedAt:   session.IssuedAt(),
			ExpiresAt:  session.ExpiresAt(),
			LastUsedAt: usedAt,
			RevokedAt:  session.RevokedAt(),
			UserAgent:  session.UserAgent(),
			IPAddress:  session.IPAddress(),
		})
	return nil
}

func (r *fakeAuthSessionRepository) RevokeByTokenHash(
	_ context.Context,
	tokenHash domainauth.SessionTokenHash,
	revokedAt time.Time,
) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, ok := r.sessions[hashKey(tokenHash)]
	if !ok || session.IsRevoked() {
		return nil
	}

	r.sessions[hashKey(tokenHash)] = domainauth.ReconstructAuthSession(
		domainauth.ReconstructAuthSessionParams{
			ID:         session.ID(),
			PublicID:   session.PublicID(),
			UserID:     session.UserID(),
			TokenHash:  session.TokenHash(),
			IssuedAt:   session.IssuedAt(),
			ExpiresAt:  session.ExpiresAt(),
			LastUsedAt: session.LastUsedAt(),
			RevokedAt:  &revokedAt,
			UserAgent:  session.UserAgent(),
			IPAddress:  session.IPAddress(),
		})
	return nil
}

func (r *fakeAuthSessionRepository) RevokeAllByUserID(
	ctx context.Context,
	id domainauth.UserID,
	revokedAt time.Time,
) (int64, error) {
	r.mutex.Lock()
	tokenHashes := make([]domainauth.SessionTokenHash, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session.UserID() == id && !session.IsRevoked() {
			tokenHashes = append(tokenHashes, session.TokenHash())
		}
	}
	r.mutex.Unlock()

	for _, tokenHash := range tokenHashes {
		if err := r.RevokeByTokenHash(ctx, tokenHash, revokedAt); err != nil {
			return 0, err
		}
	}
	return int64(len(tokenHashes)), nil
}

func (r *fakeAuthSessionRepository) DeleteExpired(
	_ context.Context,
	expiredBefore time.Time,
) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	deleted := int64(0)
	for key, session := range r.sessions {
		if !session.ExpiresAt().After(expiredBefore) {
			delete(r.sessions, key)
			deleted++
		}
	}
	return deleted, nil
}

var _ domainauth.AuthSessionRepository = (*fakeAuthSessionRepository)(nil)

// fakePasswordHasher はhash計算を伴わない決定的な実装。unit testを高速化する。
type fakePasswordHasher struct {
	mutex      sync.Mutex
	hashCount  int
	failOnHash bool
}

func (h *fakePasswordHasher) Hash(password domainauth.RawPassword) (domainauth.PasswordHash, error) {
	h.mutex.Lock()
	h.hashCount++
	h.mutex.Unlock()

	if h.failOnHash {
		return domainauth.PasswordHash{}, errRepository
	}
	return domainauth.NewPasswordHash(domainauth.PasswordHashPrefix + password.Expose())
}

func (h *fakePasswordHasher) Verify(
	password domainauth.RawPassword,
	hash domainauth.PasswordHash,
) (bool, error) {
	return hash.Encoded() == domainauth.PasswordHashPrefix+password.Expose(), nil
}

func (h *fakePasswordHasher) HashCount() int {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.hashCount
}

var _ domainauth.PasswordHasher = (*fakePasswordHasher)(nil)

// fixedSessionTokenGenerator は決まった順序でtokenを返す。
type fixedSessionTokenGenerator struct {
	mutex  sync.Mutex
	tokens []string
	index  int
}

func newFixedSessionTokenGenerator(tokens ...string) *fixedSessionTokenGenerator {
	return &fixedSessionTokenGenerator{tokens: tokens}
}

func (g *fixedSessionTokenGenerator) Generate() (domainauth.SessionToken, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.index >= len(g.tokens) {
		return domainauth.SessionToken{}, errors.New("no more tokens")
	}
	token := g.tokens[g.index]
	g.index++
	return domainauth.NewSessionToken(token)
}

var _ domainauth.SessionTokenGenerator = (*fixedSessionTokenGenerator)(nil)

// sequentialPublicIDGenerator は決定的なUUIDを返す。
type sequentialPublicIDGenerator struct {
	mutex   sync.Mutex
	counter int
	failing bool
}

func (g *sequentialPublicIDGenerator) NewPublicID() (uuid.UUID, error) {
	if g.failing {
		return uuid.Nil, errors.New("id generation failed")
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.counter++

	var generated uuid.UUID
	generated[15] = byte(g.counter)
	generated[6] = 0x70
	generated[8] = 0x80
	return generated, nil
}

// fakeCategoryRepository はmemory上でCategoryRepositoryを実装する。
//
// ユーザー登録時の既定カテゴリー作成 (設計書 28章 Phase 1) を検証するために使用する。
type fakeCategoryRepository struct {
	mutex            sync.Mutex
	categoriesByUser map[domainauth.UserID][]domaincategory.Category
	nextID           int64
	failOnCreateAll  bool
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categoriesByUser: make(map[domainauth.UserID][]domaincategory.Category),
		nextID:           1,
	}
}

func (r *fakeCategoryRepository) CreateAll(
	_ context.Context,
	categories []domaincategory.Category,
) ([]domaincategory.Category, error) {
	if r.failOnCreateAll {
		return nil, errRepository
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	created := make([]domaincategory.Category, 0, len(categories))
	for _, category := range categories {
		stored := category.WithID(domaincategory.CategoryID(r.nextID))
		r.nextID++
		r.categoriesByUser[category.UserID()] = append(r.categoriesByUser[category.UserID()], stored)
		created = append(created, stored)
	}
	return created, nil
}

func (r *fakeCategoryRepository) ListActiveByUserID(
	_ context.Context,
	userID domainauth.UserID,
) ([]domaincategory.Category, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.categoriesByUser[userID], nil
}

func (r *fakeCategoryRepository) FindActiveByPublicID(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domaincategory.Category, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, category := range r.categoriesByUser[userID] {
		if category.PublicID() == publicID {
			return category, nil
		}
	}
	return domaincategory.Category{}, domaincategory.ErrCategoryNotFound
}

var _ domaincategory.CategoryRepository = (*fakeCategoryRepository)(nil)
