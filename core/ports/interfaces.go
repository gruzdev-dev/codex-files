package ports

import "codex-files/core/domain"

type UserService interface {
	HealthCheck() string
}

type UserRepository interface {
	GetUser(id string) (domain.User, error)
}
