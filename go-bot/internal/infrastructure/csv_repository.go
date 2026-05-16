package infrastructure

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/kpfu-schedule-bot/go-bot/internal/config"
	"github.com/kpfu-schedule-bot/go-bot/internal/logger"
)

// CSVRepositoryImpl реализует интерфейс domain.CSVRepository
type CSVRepositoryImpl struct {
	cfg        *config.Config
	cacheDir   string
	groupIndex map[string]string // groupCode -> filename
	mu         sync.RWMutex
	logger     *logger.Logger
}

// NewCSVRepository создает новый экземпляр репозитория
func NewCSVRepository(cfg *config.Config, logger *logger.Logger) (*CSVRepositoryImpl, error) {
	cacheDir := cfg.CacheDir
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	repo := &CSVRepositoryImpl{
		cfg:        cfg,
		cacheDir:   cacheDir,
		groupIndex: make(map[string]string),
		logger:     logger,
	}

	return repo, nil
}

// gidPath возвращает путь к файлу для данного GID
func (r *CSVRepositoryImpl) gidPath(gid int) string {
	return filepath.Join(r.cacheDir, fmt.Sprintf("gid_%d.csv", gid))
}

// DownloadAll скачивает все CSV файлы для указанных GID
func (r *CSVRepositoryImpl) DownloadAll(ctx context.Context, gids []int) error {
	sem := make(chan struct{}, 4) // Ограничение на 4 параллельных загрузки
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, gid := range gids {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				
				if err := r.downloadGID(ctx, gid); err != nil {
					r.logger.Error("Failed to download GID", "gid", gid, "error", err)
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			case <-ctx.Done():
				return
			}
		}(gid)
	}

	wg.Wait()
	return firstErr
}

// downloadGID скачивает CSV для одного GID
func (r *CSVRepositoryImpl) downloadGID(ctx context.Context, gid int) error {
	url := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%d", 
		r.cfg.SpreadsheetID, gid)

	r.logger.Info("Downloading CSV", "gid", gid, "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	csvText := string(body)
	if csvText == "" {
		r.logger.Warn("Empty CSV received", "gid", gid)
		return nil
	}

	// Запись во временный файл, затем атомарная замена
	tmpPath := r.gidPath(gid) + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(csvText), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, r.gidPath(gid)); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	r.logger.Info("CSV saved", "gid", gid, "path", r.gidPath(gid))
	return nil
}

// RefreshAll обновляет все CSV файлы
func (r *CSVRepositoryImpl) RefreshAll(ctx context.Context) error {
	r.logger.Info("Refreshing all CSV files")
	if err := r.DownloadAll(ctx, r.cfg.GIDs); err != nil {
		return err
	}
	return r.BuildIndex()
}

// FindGroupSchedule находит расписание для группы в локальном кэше
func (r *CSVRepositoryImpl) FindGroupSchedule(ctx context.Context, groupCode string) (string, error) {
	cleanCode := normalizeGroupCode(groupCode)
	if len(cleanCode) != 7 {
		return "", fmt.Errorf("invalid group code length: %s", groupCode)
	}

	r.mu.RLock()
	filename, exists := r.groupIndex[cleanCode]
	r.mu.RUnlock()

	if !exists {
		r.logger.Warn("Group not found in index", "group", cleanCode)
		return "", nil
	}

	filePath := filepath.Join(r.cacheDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		r.logger.Error("Failed to read CSV file", "path", filePath, "error", err)
		return "", err
	}

	return string(data), nil
}

// BuildIndex строит индекс групп по всем CSV файлам
func (r *CSVRepositoryImpl) BuildIndex() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.groupIndex = make(map[string]string)

	files, err := filepath.Glob(filepath.Join(r.cacheDir, "gid_*.csv"))
	if err != nil {
		return fmt.Errorf("failed to glob files: %w", err)
	}

	for _, filePath := range files {
		if err := r.indexFile(filePath); err != nil {
			r.logger.Warn("Failed to index file", "path", filePath, "error", err)
		}
	}

	r.logger.Info("Built index", "groups", len(r.groupIndex))
	return nil
}

// indexFile индексирует один CSV файл
func (r *CSVRepositoryImpl) indexFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return err
	}

	// Ищем все 7-значные числа в заголовке
	headerStr := strings.Join(header, ",")
	allDigits := regexp.MustCompile(`\d+`).FindAllString(headerStr, -1)
	
	for _, digits := range allDigits {
		if len(digits) == 7 {
			if _, exists := r.groupIndex[digits]; !exists {
				r.groupIndex[digits] = filepath.Base(filePath)
			}
		}
	}

	return nil
}

// normalizeGroupCode нормализует код группы (оставляет только цифры)
func normalizeGroupCode(s string) string {
	var result strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// EnsureStartupCache гарантирует наличие кэша при старте
func (r *CSVRepositoryImpl) EnsureStartupCache(ctx context.Context) error {
	existing := make(map[int]bool)
	
	files, err := filepath.Glob(filepath.Join(r.cacheDir, "gid_*.csv"))
	if err != nil {
		return err
	}

	for _, f := range files {
		base := filepath.Base(f)
		// Извлекаем GID из имени файла gid_X.csv
		if n, err := fmt.Sscanf(base, "gid_%d.csv", new(int)); err == nil && n == 1 {
			var gid int
			fmt.Sscanf(base, "gid_%d.csv", &gid)
			existing[gid] = true
		}
	}

	var missing []int
	for _, gid := range r.cfg.GIDs {
		if !existing[gid] {
			missing = append(missing, gid)
		}
	}

	if len(existing) == 0 {
		r.logger.Info("Cache is empty, performing initial download", "gids", len(r.cfg.GIDs))
		return r.DownloadAll(ctx, r.cfg.GIDs)
	}

	if len(missing) > 0 {
		r.logger.Info("Missing GIDs in cache", "count", len(missing), "gids", missing)
		return r.DownloadAll(ctx, missing)
	}

	r.logger.Info("CSV cache is up to date", "files", len(existing))
	return nil
}
