package refreshtokens

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/users"
)

const (
	defaultPrefix  = "auth:refresh:"
	accessPrefix   = "auth:access:jti:"
	defaultTimeout = 3 * time.Second
)

const storeScript = `
local userKey = KEYS[1]
local newTokenKey = KEYS[2]
local newTokenJTI = ARGV[1]
local ttl = ARGV[2]
local tokenPrefix = ARGV[3]
local userID = ARGV[4]

local oldTokenJTI = redis.call("GET", userKey)
if oldTokenJTI then
  redis.call("DEL", tokenPrefix .. oldTokenJTI)
end

redis.call("SET", userKey, newTokenJTI, "EX", ttl)
redis.call("SET", newTokenKey, userID, "EX", ttl)
return 1
`

const rotateScript = `
local userKey = KEYS[1]
local oldTokenKey = KEYS[2]
local newTokenKey = KEYS[3]
local oldTokenJTI = ARGV[1]
local newTokenJTI = ARGV[2]
local ttl = ARGV[3]
local userID = ARGV[4]

local currentJTI = redis.call("GET", userKey)
if currentJTI ~= oldTokenJTI then
  return nil
end

local mappedUserID = redis.call("GET", oldTokenKey)
if mappedUserID ~= userID then
  return nil
end

redis.call("DEL", oldTokenKey)
redis.call("SET", newTokenKey, userID, "EX", ttl)
redis.call("SET", userKey, newTokenJTI, "EX", ttl)
return 1
`

const revokeRefreshScript = `
local userKey = KEYS[1]
local tokenKey = KEYS[2]
local tokenJTI = ARGV[1]

local currentJTI = redis.call("GET", userKey)
if currentJTI == tokenJTI then
  redis.call("DEL", userKey)
end

redis.call("DEL", tokenKey)
return 1
`

type Store struct {
	addr      string
	username  string
	password  string
	db        int
	useTLS    bool
	timeout   time.Duration
	keyPrefix string
}

var _ domain.RefreshTokenStore = (*Store)(nil)
var _ domain.AccessTokenStore = (*Store)(nil)

func NewStoreFromURL(rawURL string) (*Store, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse cache url: %w", err)
	}

	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return nil, fmt.Errorf("unsupported cache scheme: %s", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("cache host is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "6379"
	}

	db := 0
	if parsed.Path != "" && parsed.Path != "/" {
		dbValue := strings.TrimPrefix(parsed.Path, "/")
		db, err = strconv.Atoi(dbValue)
		if err != nil {
			return nil, fmt.Errorf("parse cache db index: %w", err)
		}
	}

	password, _ := parsed.User.Password()
	store := &Store{
		addr:      net.JoinHostPort(host, port),
		username:  parsed.User.Username(),
		password:  password,
		db:        db,
		useTLS:    parsed.Scheme == "rediss",
		timeout:   defaultTimeout,
		keyPrefix: defaultPrefix,
	}

	if _, err := store.exec(context.Background(), []string{"PING"}); err != nil {
		return nil, fmt.Errorf("ping cache: %w", err)
	}

	return store, nil
}

func (s *Store) Store(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error {
	ttlSeconds := int64(ttl / time.Second)
	if ttlSeconds <= 0 {
		return errors.New("ttl must be greater than zero")
	}

	_, err := s.execEval(
		ctx,
		storeScript,
		[]string{s.userKey(userID), s.tokenKey(refreshJTI)},
		[]string{
			refreshJTI,
			strconv.FormatInt(ttlSeconds, 10),
			s.tokenKeyPrefix(),
			userID.String(),
		},
	)
	return err
}

func (s *Store) Rotate(
	ctx context.Context,
	userID uuid.UUID,
	oldRefreshJTI,
	newRefreshJTI string,
	ttl time.Duration,
) error {
	ttlSeconds := int64(ttl / time.Second)
	if ttlSeconds <= 0 {
		return errors.New("ttl must be greater than zero")
	}

	result, err := s.execEval(
		ctx,
		rotateScript,
		[]string{
			s.userKey(userID),
			s.tokenKey(oldRefreshJTI),
			s.tokenKey(newRefreshJTI),
		},
		[]string{
			oldRefreshJTI,
			newRefreshJTI,
			strconv.FormatInt(ttlSeconds, 10),
			userID.String(),
		},
	)
	if err != nil {
		return err
	}
	if result == nil {
		return domain.ErrInvalidRefreshToken
	}
	return nil
}

func (s *Store) StoreAccessToken(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
	ttl time.Duration,
) error {
	ttlSeconds := int64(ttl / time.Second)
	if ttlSeconds <= 0 {
		return errors.New("ttl must be greater than zero")
	}

	_, err := s.exec(
		ctx,
		[]string{
			"SET",
			s.accessTokenKey(jti),
			userID.String(),
			"EX",
			strconv.FormatInt(ttlSeconds, 10),
		},
	)
	return err
}

func (s *Store) IsAccessTokenActive(ctx context.Context, jti string, userID uuid.UUID) (bool, error) {
	raw, err := s.exec(ctx, []string{"GET", s.accessTokenKey(jti)})
	if err != nil {
		return false, err
	}
	if raw == nil {
		return false, nil
	}

	storedUserID, ok := raw.(string)
	if !ok {
		return false, nil
	}

	return storedUserID == userID.String(), nil
}

func (s *Store) RevokeAccessToken(ctx context.Context, jti string) error {
	_, err := s.exec(ctx, []string{"DEL", s.accessTokenKey(jti)})
	return err
}

func (s *Store) Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error {
	_, err := s.execEval(
		ctx,
		revokeRefreshScript,
		[]string{
			s.userKey(userID),
			s.tokenKey(refreshJTI),
		},
		[]string{
			refreshJTI,
		},
	)
	return err
}

func (s *Store) execEval(ctx context.Context, script string, keys, args []string) (any, error) {
	command := []string{"EVAL", script, strconv.Itoa(len(keys))}
	command = append(command, keys...)
	command = append(command, args...)
	return s.exec(ctx, command)
}

func (s *Store) exec(ctx context.Context, command []string) (any, error) {
	conn, rw, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if s.password != "" {
		authCommand := []string{"AUTH", s.password}
		if s.username != "" {
			authCommand = []string{"AUTH", s.username, s.password}
		}
		if err := writeCommand(rw, authCommand); err != nil {
			return nil, err
		}
		if _, err := readRESP(rw.Reader); err != nil {
			return nil, err
		}
	}

	if s.db > 0 {
		if err := writeCommand(rw, []string{"SELECT", strconv.Itoa(s.db)}); err != nil {
			return nil, err
		}
		if _, err := readRESP(rw.Reader); err != nil {
			return nil, err
		}
	}

	if err := writeCommand(rw, command); err != nil {
		return nil, err
	}
	return readRESP(rw.Reader)
}

func (s *Store) dial(ctx context.Context) (net.Conn, *bufio.ReadWriter, error) {
	dialer := &net.Dialer{Timeout: s.timeout}
	var conn net.Conn
	var err error

	if s.useTLS {
		serverName, _, splitErr := net.SplitHostPort(s.addr)
		if splitErr != nil {
			return nil, nil, splitErr
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", s.addr, &tls.Config{ServerName: serverName})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", s.addr)
	}
	if err != nil {
		return nil, nil, err
	}

	_ = conn.SetDeadline(time.Now().Add(s.timeout))
	return conn, bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)), nil
}

func writeCommand(rw *bufio.ReadWriter, parts []string) error {
	if _, err := rw.WriteString(fmt.Sprintf("*%d\r\n", len(parts))); err != nil {
		return err
	}

	for _, part := range parts {
		if _, err := rw.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(part), part)); err != nil {
			return err
		}
	}

	return rw.Flush()
}

func readRESP(reader *bufio.Reader) (any, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch prefix {
	case '+':
		return readLine(reader)
	case '-':
		msg, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(msg)
	case ':':
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		value, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case '$':
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return nil, nil
		}

		buf := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*':
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return nil, nil
		}

		items := make([]any, 0, length)
		for i := 0; i < length; i++ {
			item, err := readRESP(reader)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported resp prefix: %q", prefix)
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func (s *Store) tokenKey(refreshJTI string) string {
	return s.tokenKeyPrefix() + refreshJTI
}

func (s *Store) tokenKeyPrefix() string {
	return s.keyPrefix + "token:"
}

func (s *Store) userKey(userID uuid.UUID) string {
	return s.userKeyPrefix() + userID.String()
}

func (s *Store) userKeyPrefix() string {
	return s.keyPrefix + "user:"
}

func (s *Store) accessTokenKey(jti string) string {
	return accessPrefix + jti
}
