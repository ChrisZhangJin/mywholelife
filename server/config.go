package server

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr     string
	DataRoot string
	DBPath   string
}

func FromEnv() Config {
	addr := os.Getenv("MWL_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dataRoot := os.Getenv("MWL_DATA_ROOT")
	if dataRoot == "" {
		dataRoot = "./data"
	}
	dbPath := os.Getenv("MWL_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(dataRoot, "mywholelife.db")
	}
	return Config{Addr: addr, DataRoot: dataRoot, DBPath: dbPath}
}
