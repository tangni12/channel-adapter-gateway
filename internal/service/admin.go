package service

import (
	"errors"
	"time"

	"channel-adapter-gateway/internal/model"

	"gorm.io/gorm"
)

type AdminService struct {
	db    *gorm.DB
	auth  *AuthService
	cache *MappingCache
}

func NewAdminService(db *gorm.DB, auth *AuthService, cache *MappingCache) *AdminService {
	return &AdminService{db: db, auth: auth, cache: cache}
}

func (s *AdminService) Login(username, password string) (string, *model.User, error) {
	var user model.User
	if err := s.db.Where("username = ? AND enabled = ?", username, true).First(&user).Error; err != nil {
		return "", nil, errors.New("invalid username or password")
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return "", nil, errors.New("invalid username or password")
	}
	now := time.Now()
	user.LastLoginAt = &now
	_ = s.db.Save(&user).Error
	token, err := s.auth.IssueToken(user.ID, user.Username, user.Role)
	return token, &user, err
}

func (s *AdminService) RefreshMappings() error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Refresh()
}

func (s *AdminService) FindUser(id uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
