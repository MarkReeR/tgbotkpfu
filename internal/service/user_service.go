package service

import (
	"time"
	"tgbotkpfu/internal/models"
	"tgbotkpfu/internal/repository"
	"tgbotkpfu/pkg/schedule"
)

type UserService struct {
	repo *repository.FileRepository
}

func NewUserService(repo *repository.FileRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) BindUserToGroup(userID int64, groupID string) error {
	return s.repo.SetUserGroup(userID, groupID)
}

func (s *UserService) GetUser(userID int64) (*models.User, error) {
	return s.repo.GetUser(userID)
}

func (s *UserService) ToggleNotify(userID int64, enabled bool) error {
	return s.repo.SetNotifyEnabled(userID, enabled)
}

func (s *UserService) GetScheduleForUser(userID int64) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}

	if user.GroupID == "" {
		return "Сначала привяжите группу командой /bind <номер_группы>", nil
	}

	daySchedule, err := schedule.GetScheduleForDay(time.Now().Weekday(), user.GroupID)
	if err != nil {
		return "", err
	}

	return schedule.FormatSchedule(daySchedule), nil
}

func (s *UserService) GetTomorrowSchedule(userID int64) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}

	if user.GroupID == "" {
		return "", nil
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	if tomorrow.Weekday() == time.Sunday {
		return "", nil
	}

	daySchedule, err := schedule.GetScheduleForDay(tomorrow.Weekday(), user.GroupID)
	if err != nil {
		return "", err
	}

	return schedule.FormatSchedule(daySchedule), nil
}

func (s *UserService) GetAllUsersWithNotify() ([]*models.User, error) {
	return s.repo.GetAllUsersWithNotify()
}
