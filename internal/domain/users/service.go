package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	authinfra "heimly.space/backend/internal/infra/auth"
)

type Service struct {
	Repo            Repository
	AccessTokens    AccessTokenStore
	RefreshTokens   RefreshTokenStore
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func (s *Service) Register(
	ctx context.Context,
	login, email, name, password string,
	birthday time.Time,
) (*AuthTokens, error) {
	hash, err := authinfra.HashPassword(password)
	if err != nil {
		return nil, err
	}

	userID, err := s.Repo.Create(ctx, login, email, name, hash, birthday)
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, userID)
}

func (s *Service) Login(ctx context.Context, login, password string) (*AuthTokens, error) {
	user, err := s.Repo.GetByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := authinfra.CheckPassword(user.HashedPassword, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user.ID)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	if s.RefreshTokens == nil {
		return nil, errors.New("refresh token store is not configured")
	}

	refreshClaims, err := authinfra.ParseRefreshTokenClaims(refreshToken, s.JWTSecret)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	newRefreshToken, newRefreshJTI, err := authinfra.GenerateRefreshToken(
		refreshClaims.UserID,
		s.JWTSecret,
		s.RefreshTokenTTL,
	)
	if err != nil {
		return nil, err
	}

	err = s.RefreshTokens.Rotate(
		ctx,
		refreshClaims.UserID,
		refreshClaims.JTI,
		newRefreshJTI,
		s.RefreshTokenTTL,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	if s.AccessTokens == nil {
		return nil, errors.New("access token store is not configured")
	}
	accessToken, accessJTI, err := authinfra.GenerateTokenWithJTI(
		refreshClaims.UserID,
		s.JWTSecret,
		s.AccessTokenTTL,
	)
	if err != nil {
		return nil, err
	}
	if err := s.AccessTokens.StoreAccessToken(
		ctx,
		accessJTI,
		refreshClaims.UserID,
		s.AccessTokenTTL,
	); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken, accessJTI string) error {
	if s.RefreshTokens == nil {
		return errors.New("refresh token store is not configured")
	}
	if s.AccessTokens == nil {
		return errors.New("access token store is not configured")
	}

	if refreshToken != "" {
		refreshClaims, err := authinfra.ParseRefreshTokenClaims(refreshToken, s.JWTSecret)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if err := s.RefreshTokens.Revoke(ctx, refreshClaims.UserID, refreshClaims.JTI); err != nil {
			return err
		}
	}
	if accessJTI != "" {
		if err := s.AccessTokens.RevokeAccessToken(ctx, accessJTI); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.Repo.GetByID(ctx, id)
}

type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

func (s *Service) issueTokens(ctx context.Context, userID uuid.UUID) (*AuthTokens, error) {
	if s.AccessTokens == nil {
		return nil, errors.New("access token store is not configured")
	}
	if s.RefreshTokens == nil {
		return nil, errors.New("refresh token store is not configured")
	}

	accessToken, accessJTI, err := authinfra.GenerateTokenWithJTI(userID, s.JWTSecret, s.AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	if err := s.AccessTokens.StoreAccessToken(ctx, accessJTI, userID, s.AccessTokenTTL); err != nil {
		return nil, err
	}

	refreshToken, refreshJTI, err := authinfra.GenerateRefreshToken(userID, s.JWTSecret, s.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	if err := s.RefreshTokens.Store(
		ctx,
		userID,
		refreshJTI,
		s.RefreshTokenTTL,
	); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
