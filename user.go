package auth_service

import (
	"errors"
	"github.com/google/uuid"
	"net/mail"
	"sync"
)

type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

var (
	ErrorInvalidEmail = errors.New("invalid email")
)

func NewUser(email string) (*User, error) {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return nil, ErrorInvalidEmail
	}

	return &User{
		ID:    uuid.New(),
		Email: email,
	}, nil
}

type UserDataStore interface {
	GetUser(email string) (*User, error)
	CreateUser(user *User) error
}

type MapDataStore struct {
	users sync.Map
}

func NewMapDataStore() *MapDataStore {
	return &MapDataStore{
		users: sync.Map{},
	}
}

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

func (store *MapDataStore) GetUser(email string) (*User, error) {
	if user, ok := store.users.Load(email); ok {
		return user.(*User), nil
	}

	return nil, ErrUserNotFound
}

func (store *MapDataStore) CreateUser(user *User) error {
	if _, err := store.GetUser(user.Email); err == nil {
		return ErrUserAlreadyExists
	}

	store.users.Store(user.Email, user)
	return nil
}
