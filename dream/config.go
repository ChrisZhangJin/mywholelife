package dream

import (
	"os"
	"path/filepath"
	"strconv"
)

const day = int64(86400)

// Config holds the dream job's thresholds (stored as seconds so the job can
// compare against int64 now-access_time directly) plus the LLM client settings.
type Config struct {
	DataRoot     string
	DBPath       string
	LLMBaseURL   string
	LLMAPIKey    string
	Model        string
	T1           int64
	T2           int64
	T3           int64
	Grace        int64
	MaxDeletions int
}

func FromEnv() Config {
	dataRoot := os.Getenv("MWL_DATA_ROOT")
	if dataRoot == "" {
		dataRoot = "./data"
	}
	dbPath := os.Getenv("MWL_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(dataRoot, "mywholelife.db")
	}
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	model := os.Getenv("DREAM_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return Config{
		DataRoot:     dataRoot,
		DBPath:       dbPath,
		LLMBaseURL:   baseURL,
		LLMAPIKey:    os.Getenv("LLM_API_KEY"),
		Model:        model,
		T1:           envDays("T1", 14),
		T2:           envDays("T2", 90),
		T3:           envDays("T3", 180),
		Grace:        envDays("GRACE", 30),
		MaxDeletions: envInt("DREAM_MAX_DELETIONS", 50),
	}
}

func envDays(key string, def int64) int64 {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return int64(v) * day
	}
	return def * day
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
