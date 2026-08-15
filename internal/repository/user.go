package repository

import (
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Upsert(id, username, email string, avatar *string) (*model.User, error) {
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	user := &model.User{ID: id, Username: username, Email: email, Avatar: avatar}
	err := r.db.Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"username": username,
			"email":    email,
			"avatar":   avatar,
		}).Error
	if err != nil {
		return nil, err
	}
	if err := r.db.First(user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// UpsertByGoogle finds or creates a user by their Google subject id, then
// refreshes the display fields. The user id is stable as "google_" + sub so
// chats and messages keep working across sessions. The insert is atomic
// (ON CONFLICT ... DO UPDATE) so concurrent authenticated requests cannot
// race on the primary key.
func (r *UserRepo) UpsertByGoogle(sub, username, email, avatar string) (*model.User, error) {
	var avatarPtr *string
	if avatar != "" {
		a := avatar
		avatarPtr = &a
	}

	user := model.User{
		ID:           "google_" + sub,
		Username:     username,
		Email:        email,
		Avatar:       avatarPtr,
		GoogleSub:    sub,
		AuthProvider: "google",
	}

	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"username", "email", "avatar"}),
	}).Create(&user).Error
	if err != nil {
		return nil, err
	}

	var saved model.User
	if err := r.db.Where("id = ?", user.ID).First(&saved).Error; err != nil {
		return nil, err
	}
	return &saved, nil
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
