package token

import (
	"time"
)

type Maker interface {
	CreateToken(email string, kind Kind, duration time.Duration) (string, *Payload, error)
	VerifyToken(token string) (*Payload, error)
}
