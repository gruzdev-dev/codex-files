package storage

import (
	"codex-files/core/domain"
	"codex-files/core/ports"
	"fmt"
)

type inMemoryRepo struct {
	users map[string]domain.User
}

func NewInMemoryRepo() ports.UserRepository {
	return &inMemoryRepo{
		users: make(map[string]domain.User),
	}
}

func (r *inMemoryRepo) GetUser(id string) (domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("user not found")
	}
	return user, nil
}
