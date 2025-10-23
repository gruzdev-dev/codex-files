package service

import "codex-files/core/ports"

type userService struct {
	repo ports.UserRepository
}

func NewUserService(repo ports.UserRepository) ports.UserService {
	return &userService{
		repo: repo,
	}
}

func (s *userService) HealthCheck() string {
	return "Core Service is OK"
}
