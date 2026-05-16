package repository

import (
	"encoding/json"
	"os"
	"sync"
	"tgbotkpfu/internal/models"
)

type FileRepository struct {
	filePath string
	mu       sync.RWMutex
	users    map[int64]*models.User
}

func NewFileRepository(filePath string) (*FileRepository, error) {
	repo := &FileRepository{
		filePath: filePath,
		users:    make(map[int64]*models.User),
	}
	if err := repo.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return repo, nil
}

func (r *FileRepository) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	var users []*models.User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	for _, u := range users {
		r.users[u.ID] = u
	}
	return nil
}

func (r *FileRepository) save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*models.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.filePath, data, 0644)
}

func (r *FileRepository) GetUser(id int64) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return &models.User{ID: id, NotifyEnabled: true}, nil
}

func (r *FileRepository) SaveUser(user *models.User) error {
	r.mu.Lock()
	r.users[user.ID] = user
	r.mu.Unlock()

	return r.save()
}

func (r *FileRepository) SetUserGroup(id int64, groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		user = &models.User{ID: id, NotifyEnabled: true}
		r.users[id] = user
	}
	user.GroupID = groupID

	return r.save()
}

func (r *FileRepository) SetNotifyEnabled(id int64, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		user = &models.User{ID: id, GroupID: ""}
		r.users[id] = user
	}
	user.NotifyEnabled = enabled

	return r.save()
}

func (r *FileRepository) GetAllUsersWithNotify() ([]*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*models.User
	for _, user := range r.users {
		if user.NotifyEnabled && user.GroupID != "" {
			result = append(result, user)
		}
	}
	return result, nil
}
