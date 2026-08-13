package repository

import (
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Upsert(id, username, email string, avatar *string) (*model.User, error) {
	user := &model.User{ID: id, Username: username, Email: email, Avatar: avatar}
	err := r.db.Save(user).Error
	return user, err
}

func (r *UserRepo) GetAll() ([]model.User, error) {
	users := make([]model.User, 0)
	err := r.db.Order("username").Find(&users).Error
	return users, err
}

func (r *UserRepo) Search(q string) ([]model.User, error) {
	users := make([]model.User, 0)
	err := r.db.Where("username ILIKE ? OR email ILIKE ?", "%"+q+"%", "%"+q+"%").
		Limit(20).Find(&users).Error
	return users, err
}

func (r *UserRepo) GetByID(id string) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, "id = ?", id).Error
	return &user, err
}

func (r *UserRepo) Delete(id string) error {
	return r.db.Delete(&model.User{}, "id = ?", id).Error
}
