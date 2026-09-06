package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"heimly.space/backend/internal/cfg"
	householddomain "heimly.space/backend/internal/domain/households"
	taskdomain "heimly.space/backend/internal/domain/tasks"
	domain "heimly.space/backend/internal/domain/users"
	authinfra "heimly.space/backend/internal/infra/auth"
	householdshttp "heimly.space/backend/internal/transport/http/households"
	taskhttp "heimly.space/backend/internal/transport/http/tasks"
	usershttp "heimly.space/backend/internal/transport/http/users"
)

type routerRepoStub struct {
	createFn     func(ctx context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error)
	getByLoginFn func(ctx context.Context, login string) (*domain.UserWithPassword, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func (r *routerRepoStub) Create(
	ctx context.Context,
	login, email, name, hash string,
	birthday time.Time,
) (uuid.UUID, error) {
	if r.createFn == nil {
		return uuid.Nil, errors.New("unexpected Create call")
	}
	return r.createFn(ctx, login, email, name, hash, birthday)
}

func (r *routerRepoStub) GetByLogin(ctx context.Context, login string) (*domain.UserWithPassword, error) {
	if r.getByLoginFn == nil {
		return nil, errors.New("unexpected GetByLogin call")
	}
	return r.getByLoginFn(ctx, login)
}

func (r *routerRepoStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if r.getByIDFn == nil {
		return nil, errors.New("unexpected GetByID call")
	}
	return r.getByIDFn(ctx, id)
}

type routerRefreshStoreStub struct {
	storeFn  func(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error
	rotateFn func(ctx context.Context, userID uuid.UUID, oldRefreshJTI, newRefreshJTI string, ttl time.Duration) error
	revokeFn func(ctx context.Context, userID uuid.UUID, refreshJTI string) error
}

func (s *routerRefreshStoreStub) Store(
	ctx context.Context,
	userID uuid.UUID,
	refreshJTI string,
	ttl time.Duration,
) error {
	if s.storeFn == nil {
		return errors.New("unexpected Store call")
	}
	return s.storeFn(ctx, userID, refreshJTI, ttl)
}

func (s *routerRefreshStoreStub) Rotate(
	ctx context.Context,
	userID uuid.UUID,
	oldRefreshJTI, newRefreshJTI string,
	ttl time.Duration,
) error {
	if s.rotateFn == nil {
		return errors.New("unexpected Rotate call")
	}
	return s.rotateFn(ctx, userID, oldRefreshJTI, newRefreshJTI, ttl)
}

func (s *routerRefreshStoreStub) Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected Revoke call")
	}
	return s.revokeFn(ctx, userID, refreshJTI)
}

type routerAccessStoreStub struct {
	storeFn    func(ctx context.Context, jti string, userID uuid.UUID, ttl time.Duration) error
	isActiveFn func(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
	revokeFn   func(ctx context.Context, jti string) error
}

func (s *routerAccessStoreStub) StoreAccessToken(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
	ttl time.Duration,
) error {
	if s.storeFn == nil {
		return errors.New("unexpected StoreAccessToken call")
	}
	return s.storeFn(ctx, jti, userID, ttl)
}

func (s *routerAccessStoreStub) IsAccessTokenActive(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
) (bool, error) {
	if s.isActiveFn == nil {
		return false, errors.New("unexpected IsAccessTokenActive call")
	}
	return s.isActiveFn(ctx, jti, userID)
}

func (s *routerAccessStoreStub) RevokeAccessToken(ctx context.Context, jti string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected RevokeAccessToken call")
	}
	return s.revokeFn(ctx, jti)
}

type routerHouseholdRepoStub struct {
	createFn           func(ctx context.Context, name string, ownerID uuid.UUID) (*householddomain.Household, error)
	existsFn           func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isMemberFn         func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	addMemberByEmailFn func(ctx context.Context, householdID uuid.UUID, email string) (*householddomain.Member, error)
	listMembersFn      func(
		ctx context.Context,
		householdID uuid.UUID,
		cursor *householddomain.MembersListCursor,
		limit int,
	) ([]householddomain.Member, error)
	listByUserFn func(
		ctx context.Context,
		userID uuid.UUID,
		cursor *householddomain.ListCursor,
		limit int,
	) ([]householddomain.HouseholdWithRole, error)
}

func (r *routerHouseholdRepoStub) Create(
	ctx context.Context,
	name string,
	ownerID uuid.UUID,
) (*householddomain.Household, error) {
	if r.createFn == nil {
		return nil, errors.New("unexpected Create household call")
	}
	return r.createFn(ctx, name, ownerID)
}

func (r *routerHouseholdRepoStub) Exists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	if r.existsFn == nil {
		return false, errors.New("unexpected Exists household call")
	}
	return r.existsFn(ctx, householdID)
}

func (r *routerHouseholdRepoStub) IsMember(
	ctx context.Context,
	householdID, userID uuid.UUID,
) (bool, error) {
	if r.isMemberFn == nil {
		return false, errors.New("unexpected IsMember household call")
	}
	return r.isMemberFn(ctx, householdID, userID)
}

func (r *routerHouseholdRepoStub) AddMemberByEmail(
	ctx context.Context,
	householdID uuid.UUID,
	email string,
) (*householddomain.Member, error) {
	if r.addMemberByEmailFn == nil {
		return nil, errors.New("unexpected AddMemberByEmail household call")
	}
	return r.addMemberByEmailFn(ctx, householdID, email)
}

func (r *routerHouseholdRepoStub) ListMembers(
	ctx context.Context,
	householdID uuid.UUID,
	cursor *householddomain.MembersListCursor,
	limit int,
) ([]householddomain.Member, error) {
	if r.listMembersFn == nil {
		return nil, errors.New("unexpected ListMembers household call")
	}
	return r.listMembersFn(ctx, householdID, cursor, limit)
}

func (r *routerHouseholdRepoStub) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursor *householddomain.ListCursor,
	limit int,
) ([]householddomain.HouseholdWithRole, error) {
	if r.listByUserFn == nil {
		return nil, errors.New("unexpected ListByUser household call")
	}
	return r.listByUserFn(ctx, userID, cursor, limit)
}

type routerTaskRepoStub struct {
	householdExistsFn   func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isHouseholdMemberFn func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	createFn            func(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	listByHouseholdFn   func(
		ctx context.Context,
		householdID uuid.UUID,
		filter taskdomain.ListFilter,
		cursor *taskdomain.TaskListCursor,
		limit int,
	) ([]taskdomain.Task, error)
	getByIDFn func(ctx context.Context, taskID uuid.UUID) (*taskdomain.Task, error)
	updateFn  func(ctx context.Context, task *taskdomain.Task, assigneesChanged bool) (*taskdomain.Task, error)
	deleteFn  func(ctx context.Context, taskID uuid.UUID) (bool, error)
}

func (r *routerTaskRepoStub) HouseholdExists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	if r.householdExistsFn == nil {
		return false, errors.New("unexpected task HouseholdExists call")
	}
	return r.householdExistsFn(ctx, householdID)
}

func (r *routerTaskRepoStub) IsHouseholdMember(
	ctx context.Context,
	householdID, userID uuid.UUID,
) (bool, error) {
	if r.isHouseholdMemberFn == nil {
		return false, errors.New("unexpected task IsHouseholdMember call")
	}
	return r.isHouseholdMemberFn(ctx, householdID, userID)
}

func (r *routerTaskRepoStub) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	if r.createFn == nil {
		return nil, errors.New("unexpected task Create call")
	}
	return r.createFn(ctx, task)
}

func (r *routerTaskRepoStub) ListByHousehold(
	ctx context.Context,
	householdID uuid.UUID,
	filter taskdomain.ListFilter,
	cursor *taskdomain.TaskListCursor,
	limit int,
) ([]taskdomain.Task, error) {
	if r.listByHouseholdFn == nil {
		return nil, errors.New("unexpected task ListByHousehold call")
	}
	return r.listByHouseholdFn(ctx, householdID, filter, cursor, limit)
}

func (r *routerTaskRepoStub) GetByID(ctx context.Context, taskID uuid.UUID) (*taskdomain.Task, error) {
	if r.getByIDFn == nil {
		return nil, errors.New("unexpected task GetByID call")
	}
	return r.getByIDFn(ctx, taskID)
}

func (r *routerTaskRepoStub) Update(
	ctx context.Context,
	task *taskdomain.Task,
	assigneesChanged bool,
) (*taskdomain.Task, error) {
	if r.updateFn == nil {
		return nil, errors.New("unexpected task Update call")
	}
	return r.updateFn(ctx, task, assigneesChanged)
}

func (r *routerTaskRepoStub) Delete(ctx context.Context, taskID uuid.UUID) (bool, error) {
	if r.deleteFn == nil {
		return false, errors.New("unexpected task Delete call")
	}
	return r.deleteFn(ctx, taskID)
}

func newTestRouter(
	secret string,
	repo domain.Repository,
	accessStore domain.AccessTokenStore,
	refreshStore domain.RefreshTokenStore,
) nethttp.Handler {
	authHandlers := &usershttp.AuthHandlers{
		Users: &domain.Service{
			Repo:            repo,
			AccessTokens:    accessStore,
			RefreshTokens:   refreshStore,
			JWTSecret:       secret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
	householdHandlers := &householdshttp.Handlers{
		Households: &householddomain.Service{
			CursorSecret: secret,
			Repo: &routerHouseholdRepoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*householddomain.Household, error) {
					return nil, errors.New("unexpected household create")
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected household exists")
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected household is-member")
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*householddomain.Member, error) {
					return nil, errors.New("unexpected household invite")
				},
				listMembersFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ *householddomain.MembersListCursor,
					_ int,
				) ([]householddomain.Member, error) {
					return nil, errors.New("unexpected household list")
				},
				listByUserFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ *householddomain.ListCursor,
					_ int,
				) ([]householddomain.HouseholdWithRole, error) {
					return nil, errors.New("unexpected household list-by-user")
				},
			},
		},
	}
	taskHandlers := &taskhttp.Handlers{
		Tasks: &taskdomain.Service{
			CursorSecret: secret,
			Repo: &routerTaskRepoStub{
				householdExistsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task household exists")
				},
				isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task household member")
				},
				createFn: func(_ context.Context, _ *taskdomain.Task) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task create")
				},
				listByHouseholdFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ taskdomain.ListFilter,
					_ *taskdomain.TaskListCursor,
					_ int,
				) ([]taskdomain.Task, error) {
					return nil, errors.New("unexpected task list")
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task get by id")
				},
				updateFn: func(_ context.Context, _ *taskdomain.Task, _ bool) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task update")
				},
				deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task delete")
				},
			},
		},
	}
	return NewRouter(authHandlers, householdHandlers, taskHandlers, &cfg.Config{JWTSecret: secret})
}

func newTestRouterWithHouseholdsRepo(
	secret string,
	householdRepo householddomain.Repository,
	accessStore domain.AccessTokenStore,
) nethttp.Handler {
	authHandlers := &usershttp.AuthHandlers{
		Users: &domain.Service{
			Repo: &routerRepoStub{
				createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
					return uuid.Nil, errors.New("unexpected auth create")
				},
				getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
					return nil, errors.New("unexpected auth get-by-login")
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
					return nil, errors.New("unexpected auth get-by-id")
				},
			},
			AccessTokens:    accessStore,
			RefreshTokens:   &routerRefreshStoreStub{},
			JWTSecret:       secret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
	householdHandlers := &householdshttp.Handlers{
		Households: &householddomain.Service{
			Repo:         householdRepo,
			CursorSecret: secret,
		},
	}
	taskHandlers := &taskhttp.Handlers{
		Tasks: &taskdomain.Service{
			CursorSecret: secret,
			Repo: &routerTaskRepoStub{
				householdExistsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task household exists")
				},
				isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task household member")
				},
				createFn: func(_ context.Context, _ *taskdomain.Task) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task create")
				},
				listByHouseholdFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ taskdomain.ListFilter,
					_ *taskdomain.TaskListCursor,
					_ int,
				) ([]taskdomain.Task, error) {
					return nil, errors.New("unexpected task list")
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task get by id")
				},
				updateFn: func(_ context.Context, _ *taskdomain.Task, _ bool) (*taskdomain.Task, error) {
					return nil, errors.New("unexpected task update")
				},
				deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected task delete")
				},
			},
		},
	}
	return NewRouter(authHandlers, householdHandlers, taskHandlers, &cfg.Config{JWTSecret: secret})
}

func newTestRouterWithTasksRepo(
	secret string,
	taskRepo taskdomain.Repository,
	accessStore domain.AccessTokenStore,
) nethttp.Handler {
	authHandlers := &usershttp.AuthHandlers{
		Users: &domain.Service{
			Repo: &routerRepoStub{
				createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
					return uuid.Nil, errors.New("unexpected auth create")
				},
				getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
					return nil, errors.New("unexpected auth get-by-login")
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
					return nil, errors.New("unexpected auth get-by-id")
				},
			},
			AccessTokens:    accessStore,
			RefreshTokens:   &routerRefreshStoreStub{},
			JWTSecret:       secret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
	householdHandlers := &householdshttp.Handlers{
		Households: &householddomain.Service{
			CursorSecret: secret,
			Repo: &routerHouseholdRepoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*householddomain.Household, error) {
					return nil, errors.New("unexpected household create")
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected household exists")
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					return false, errors.New("unexpected household is-member")
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*householddomain.Member, error) {
					return nil, errors.New("unexpected household invite")
				},
				listMembersFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ *householddomain.MembersListCursor,
					_ int,
				) ([]householddomain.Member, error) {
					return nil, errors.New("unexpected household list")
				},
				listByUserFn: func(
					_ context.Context,
					_ uuid.UUID,
					_ *householddomain.ListCursor,
					_ int,
				) ([]householddomain.HouseholdWithRole, error) {
					return nil, errors.New("unexpected household list-by-user")
				},
			},
		},
	}
	taskHandlers := &taskhttp.Handlers{
		Tasks: &taskdomain.Service{
			Repo:         taskRepo,
			CursorSecret: secret,
		},
	}
	return NewRouter(authHandlers, householdHandlers, taskHandlers, &cfg.Config{JWTSecret: secret})
}

func TestRouterHealth(t *testing.T) {
	router := newTestRouter("secret", &routerRepoStub{}, &routerAccessStoreStub{}, &routerRefreshStoreStub{})

	req := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected health response: %+v", body)
	}
}

func TestRouterLoginRoute(t *testing.T) {
	userID := uuid.New()
	passwordHash, err := authinfra.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var gotRefreshJTI string
	router := newTestRouter(
		"secret",
		&routerRepoStub{
			getByLoginFn: func(_ context.Context, login string) (*domain.UserWithPassword, error) {
				if login != "john" {
					t.Fatalf("unexpected login: %s", login)
				}
				return &domain.UserWithPassword{
					User:           domain.User{ID: userID, Login: "john"},
					HashedPassword: passwordHash,
				}, nil
			},
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, gotUserID uuid.UUID, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in login route")
				return false, nil
			},
		},
		&routerRefreshStoreStub{
			storeFn: func(_ context.Context, gotUserID uuid.UUID, refreshJTI string, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				gotRefreshJTI = refreshJTI
				return nil
			},
		},
	)

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"login":"john","password":"secret-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp usershttp.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "secret")
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}
	if gotRefreshJTI != refreshClaims.JTI {
		t.Fatalf("stored refresh jti mismatch: got %s want %s", gotRefreshJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}
	if _, err := authinfra.ParseToken(resp.AccessToken, "secret"); err != nil {
		t.Fatalf("access token should be valid: %v", err)
	}
}

func TestRouterRefreshRoute(t *testing.T) {
	userID := uuid.New()
	oldRefresh, oldRefreshJTI, err := authinfra.GenerateRefreshToken(userID, "secret", 24*time.Hour)
	if err != nil {
		t.Fatalf("generate old refresh token: %v", err)
	}
	var gotRotateUserID uuid.UUID
	var gotOldJTI string
	var gotNewJTI string

	router := newTestRouter(
		"secret",
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, gotUserID uuid.UUID, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in refresh route")
				return false, nil
			},
		},
		&routerRefreshStoreStub{
			rotateFn: func(
				_ context.Context,
				gotUserID uuid.UUID,
				oldRefreshJTI, newRefreshJTI string,
				_ time.Duration,
			) error {
				gotRotateUserID = gotUserID
				gotOldJTI = oldRefreshJTI
				gotNewJTI = newRefreshJTI
				return nil
			},
		},
	)

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+oldRefresh+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotRotateUserID != userID {
		t.Fatalf("unexpected rotate user id: got %s want %s", gotRotateUserID, userID)
	}
	if gotOldJTI != oldRefreshJTI {
		t.Fatalf("old refresh jti mismatch: got %s want %s", gotOldJTI, oldRefreshJTI)
	}

	var resp usershttp.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	newRefreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "secret")
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}
	if gotNewJTI != newRefreshClaims.JTI {
		t.Fatalf("new refresh jti mismatch: got %s want %s", gotNewJTI, newRefreshClaims.JTI)
	}
	if newRefreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", newRefreshClaims.UserID, userID)
	}
}

func TestRouterLogoutRoute(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	var accessToken string
	refreshToken, refreshJTI, err := authinfra.GenerateRefreshToken(userID, secret, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	var gotRevokedRefreshUserID uuid.UUID
	var gotRevokedRefreshJTI string
	var gotRevokedJTI string

	router := newTestRouter(
		secret,
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called in logout route")
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in logout route")
				return false, nil
			},
			revokeFn: func(_ context.Context, jti string) error {
				gotRevokedJTI = jti
				return nil
			},
		},
		&routerRefreshStoreStub{
			revokeFn: func(_ context.Context, gotUserID uuid.UUID, gotRefreshJTI string) error {
				gotRevokedRefreshUserID = gotUserID
				gotRevokedRefreshJTI = gotRefreshJTI
				return nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token
	claims, err := authinfra.ParseTokenClaims(accessToken, secret)
	if err != nil {
		t.Fatalf("parse token claims: %v", err)
	}

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"`+refreshToken+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotRevokedRefreshUserID != userID {
		t.Fatalf("unexpected revoked refresh user id: got %s want %s", gotRevokedRefreshUserID, userID)
	}
	if gotRevokedRefreshJTI != refreshJTI {
		t.Fatalf("unexpected revoked refresh jti: got %s want %s", gotRevokedRefreshJTI, refreshJTI)
	}
	if gotRevokedJTI != claims.JTI {
		t.Fatalf("unexpected revoked access jti: %s", gotRevokedJTI)
	}
}

func TestRouterUsersMeRouteProtected(t *testing.T) {
	router := newTestRouter(
		"secret",
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called")
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called without bearer token")
				return false, nil
			},
		},
		&routerRefreshStoreStub{},
	)

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing token") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRouterUsersMeRouteAuthorized(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	var accessToken string
	router := newTestRouter(
		secret,
		&routerRepoStub{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.User, error) {
				if id != userID {
					t.Fatalf("unexpected id: %s", id)
				}
				return &domain.User{
					ID:       userID,
					Login:    "john",
					Email:    "john@example.com",
					Name:     "John Doe",
					Birthday: time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called in protected route")
				return nil
			},
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
		&routerRefreshStoreStub{},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp usershttp.UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if resp.ID != userID.String() {
		t.Fatalf("unexpected profile id: %s", resp.ID)
	}
}

func TestRouterCreateHouseholdRoute(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	householdID := uuid.New()
	createdAt := time.Date(2026, time.February, 24, 12, 0, 0, 0, time.UTC)
	var accessToken string

	router := newTestRouterWithHouseholdsRepo(
		secret,
		&routerHouseholdRepoStub{
			createFn: func(_ context.Context, name string, ownerID uuid.UUID) (*householddomain.Household, error) {
				if name != "Wonderland Flat" {
					t.Fatalf("unexpected household name: %q", name)
				}
				if ownerID != userID {
					t.Fatalf("unexpected owner id: %s", ownerID)
				}
				return &householddomain.Household{
					ID:        householdID,
					Name:      name,
					CreatedAt: createdAt,
				}, nil
			},
			existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("Exists should not be called")
				return false, nil
			},
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
				t.Fatal("IsMember should not be called")
				return false, nil
			},
			addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*householddomain.Member, error) {
				t.Fatal("AddMemberByEmail should not be called")
				return nil, nil
			},
			listMembersFn: func(
				_ context.Context,
				_ uuid.UUID,
				_ *householddomain.MembersListCursor,
				_ int,
			) ([]householddomain.Member, error) {
				t.Fatal("ListMembers should not be called")
				return nil, nil
			},
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called")
				return nil
			},
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/households",
		bytes.NewBufferString(`{"name":"Wonderland Flat"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp householdshttp.HouseholdResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode create household response: %v", err)
	}
	if resp.ID != householdID.String() {
		t.Fatalf("unexpected household id: %s", resp.ID)
	}
	if resp.Name != "Wonderland Flat" {
		t.Fatalf("unexpected household name: %s", resp.Name)
	}
	if !resp.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected created_at: %s", resp.CreatedAt)
	}
}

func TestRouterListHouseholdsRoute(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	var accessToken string
	householdID := uuid.New()
	createdAt := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)

	router := newTestRouterWithHouseholdsRepo(
		secret,
		&routerHouseholdRepoStub{
			createFn: func(_ context.Context, _ string, _ uuid.UUID) (*householddomain.Household, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("Exists should not be called")
				return false, nil
			},
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
				t.Fatal("IsMember should not be called")
				return false, nil
			},
			addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*householddomain.Member, error) {
				t.Fatal("AddMemberByEmail should not be called")
				return nil, nil
			},
			listMembersFn: func(
				_ context.Context,
				_ uuid.UUID,
				_ *householddomain.MembersListCursor,
				_ int,
			) ([]householddomain.Member, error) {
				t.Fatal("ListMembers should not be called")
				return nil, nil
			},
			listByUserFn: func(
				_ context.Context,
				gotUserID uuid.UUID,
				cursor *householddomain.ListCursor,
				limit int,
			) ([]householddomain.HouseholdWithRole, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				if cursor != nil {
					t.Fatalf("unexpected cursor: %+v", cursor)
				}
				if limit != 11 {
					t.Fatalf("unexpected limit: %d", limit)
				}
				return []householddomain.HouseholdWithRole{
					{ID: householdID, Name: "Wonderland Flat", Role: "owner", CreatedAt: createdAt},
				}, nil
			},
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called")
				return nil
			},
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/households?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp householdshttp.HouseholdsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list households response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != householdID.String() || resp.Items[0].Role != "owner" {
		t.Fatalf("unexpected item: %+v", resp.Items[0])
	}
}

func TestRouterCreateTaskRoute(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	taskID := uuid.New()
	secret := "secret"
	var accessToken string

	router := newTestRouterWithTasksRepo(
		secret,
		&routerTaskRepoStub{
			householdExistsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID, nil
			},
			isHouseholdMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID && gotUserID == userID, nil
			},
			createFn: func(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
				if task.Title != "Serve tea at the Mad Tea Party" {
					t.Fatalf("unexpected title: %q", task.Title)
				}
				task.ID = taskID
				task.CreatedAt = time.Now().UTC()
				task.UpdatedAt = task.CreatedAt
				return task, nil
			},
			listByHouseholdFn: func(
				_ context.Context,
				_ uuid.UUID,
				_ taskdomain.ListFilter,
				_ *taskdomain.TaskListCursor,
				_ int,
			) ([]taskdomain.Task, error) {
				t.Fatal("ListByHousehold should not be called")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*taskdomain.Task, error) {
				t.Fatal("GetByID should not be called")
				return nil, nil
			},
			updateFn: func(_ context.Context, _ *taskdomain.Task, _ bool) (*taskdomain.Task, error) {
				t.Fatal("Update should not be called")
				return nil, nil
			},
			deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("Delete should not be called")
				return false, nil
			},
		},
		&routerAccessStoreStub{
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/households/"+householdID.String()+"/tasks",
		bytes.NewBufferString(`{"title":"Serve tea at the Mad Tea Party","status":"pending"}`),
	)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp taskhttp.TaskResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != taskID.String() {
		t.Fatalf("unexpected task id: %s", resp.ID)
	}
}

func TestRouterPatchTaskRoute(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	taskID := uuid.New()
	secret := "secret"
	var accessToken string

	router := newTestRouterWithTasksRepo(
		secret,
		&routerTaskRepoStub{
			householdExistsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("HouseholdExists should not be called")
				return false, nil
			},
			isHouseholdMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID && gotUserID == userID, nil
			},
			createFn: func(_ context.Context, _ *taskdomain.Task) (*taskdomain.Task, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			listByHouseholdFn: func(
				_ context.Context,
				_ uuid.UUID,
				_ taskdomain.ListFilter,
				_ *taskdomain.TaskListCursor,
				_ int,
			) ([]taskdomain.Task, error) {
				t.Fatal("ListByHousehold should not be called")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, gotTaskID uuid.UUID) (*taskdomain.Task, error) {
				if gotTaskID != taskID {
					t.Fatalf("unexpected task id: %s", gotTaskID)
				}
				return &taskdomain.Task{
					ID:          taskID,
					HouseholdID: householdID,
					Title:       "Old Rabbit Reminder",
					Status:      taskdomain.StatusPending,
					AssigneeIDs: []uuid.UUID{uuid.New()},
					CreatedAt:   time.Now().UTC(),
					UpdatedAt:   time.Now().UTC(),
				}, nil
			},
			updateFn: func(_ context.Context, task *taskdomain.Task, assigneesChanged bool) (*taskdomain.Task, error) {
				if !assigneesChanged {
					t.Fatal("expected assigneesChanged=true")
				}
				if len(task.AssigneeIDs) != 0 {
					t.Fatalf("expected no assignees, got %d", len(task.AssigneeIDs))
				}
				if task.Status != taskdomain.StatusDone {
					t.Fatalf("unexpected status: %s", task.Status)
				}
				return task, nil
			},
			deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("Delete should not be called")
				return false, nil
			},
		},
		&routerAccessStoreStub{
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(
		nethttp.MethodPatch,
		"/api/v1/tasks/"+taskID.String(),
		bytes.NewBufferString(`{"status":"done","assignee_ids":null}`),
	)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestRouterListTasksRoute(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	taskID := uuid.New()
	secret := "secret"
	var accessToken string
	now := time.Date(2026, time.February, 28, 15, 0, 0, 0, time.UTC)

	router := newTestRouterWithTasksRepo(
		secret,
		&routerTaskRepoStub{
			householdExistsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID, nil
			},
			isHouseholdMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID && gotUserID == userID, nil
			},
			createFn: func(_ context.Context, _ *taskdomain.Task) (*taskdomain.Task, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			listByHouseholdFn: func(
				_ context.Context,
				_ uuid.UUID,
				filter taskdomain.ListFilter,
				cursor *taskdomain.TaskListCursor,
				limit int,
			) ([]taskdomain.Task, error) {
				if filter.Status != taskdomain.StatusPending {
					t.Fatalf("unexpected status filter: %q", filter.Status)
				}
				if cursor != nil {
					t.Fatalf("expected nil cursor, got %+v", cursor)
				}
				if limit != 21 {
					t.Fatalf("unexpected limit: %d", limit)
				}
				return []taskdomain.Task{
					{
						ID:          taskID,
						HouseholdID: householdID,
						Title:       "Paint the white roses",
						Status:      taskdomain.StatusPending,
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				}, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*taskdomain.Task, error) {
				t.Fatal("GetByID should not be called")
				return nil, nil
			},
			updateFn: func(_ context.Context, _ *taskdomain.Task, _ bool) (*taskdomain.Task, error) {
				t.Fatal("Update should not be called")
				return nil, nil
			},
			deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("Delete should not be called")
				return false, nil
			},
		},
		&routerAccessStoreStub{
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/households/"+householdID.String()+"/tasks?status=pending&limit=20",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp taskhttp.ListTasksResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != taskID.String() {
		t.Fatalf("unexpected task id: %s", resp.Items[0].ID)
	}
}

func TestRouterDeleteTaskRoute(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	taskID := uuid.New()
	secret := "secret"
	var accessToken string

	router := newTestRouterWithTasksRepo(
		secret,
		&routerTaskRepoStub{
			householdExistsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("HouseholdExists should not be called")
				return false, nil
			},
			isHouseholdMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID && gotUserID == userID, nil
			},
			createFn: func(_ context.Context, _ *taskdomain.Task) (*taskdomain.Task, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			listByHouseholdFn: func(
				_ context.Context,
				_ uuid.UUID,
				_ taskdomain.ListFilter,
				_ *taskdomain.TaskListCursor,
				_ int,
			) ([]taskdomain.Task, error) {
				t.Fatal("ListByHousehold should not be called")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, gotTaskID uuid.UUID) (*taskdomain.Task, error) {
				if gotTaskID != taskID {
					t.Fatalf("unexpected task id: %s", gotTaskID)
				}
				return &taskdomain.Task{
					ID:          taskID,
					HouseholdID: householdID,
					Title:       "Paint the white roses",
					Status:      taskdomain.StatusPending,
				}, nil
			},
			updateFn: func(_ context.Context, _ *taskdomain.Task, _ bool) (*taskdomain.Task, error) {
				t.Fatal("Update should not be called")
				return nil, nil
			},
			deleteFn: func(_ context.Context, gotTaskID uuid.UUID) (bool, error) {
				if gotTaskID != taskID {
					t.Fatalf("unexpected task id: %s", gotTaskID)
				}
				return true, nil
			},
		},
		&routerAccessStoreStub{
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(
		nethttp.MethodDelete,
		"/api/v1/tasks/"+taskID.String(),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rec.Code, rec.Body.String())
	}
}
