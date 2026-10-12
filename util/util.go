package util

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/mitchellh/go-homedir"
)

// ConnectStorm Create database connection
func ConnectStorm() *storm.DB {
	dbPath := DBPath()

	CreateDirIfNotExist(filepath.Dir(dbPath))

	db, openErr := storm.Open(dbPath)
	FatalIfError(openErr, "Could not connect Embedded Database File")

	return db
}

// DBPath resolves the configured database path, including a stable fallback
// in the system temporary directory when the home directory is unavailable.
func DBPath() string {
	if configured := GetEnvStr("DB_FILE", ""); configured != "" {
		if expanded, err := homedir.Expand(configured); err == nil {
			configured = expanded
		}
		if absolute, err := filepath.Abs(configured); err == nil {
			return absolute
		}
		return configured
	}

	if defaultDB, err := homedir.Expand("~/.geek-life/default.db"); err == nil {
		return defaultDB
	}
	return filepath.Join(os.TempDir(), "geek-life", "default.db")
}

// DataDir returns the directory that contains Geek-life's database and
// associated files, such as task image attachments.
func DataDir() string {
	return filepath.Dir(DBPath())
}

// CreateDirIfNotExist creates a directory if not found
func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

// UnixToTime create time.Time from string timestamp
func UnixToTime(timestamp string) time.Time {
	parts := strings.Split(timestamp, ".")
	i, err := strconv.ParseInt(parts[0], 10, 64)
	if err == nil {
		return time.Unix(i, 0)
	}
	LogIfError(err, "Could not parse timestamp %s (using current time instead)", timestamp)
	return time.Now()
}

// LogIfError logs the error and returns true on Error. think as IfError
func LogIfError(err error, msgOrPattern string, args ...interface{}) bool {
	if err != nil {
		message := fmt.Sprintf(msgOrPattern, args...)
		log.Printf("%s: %v\n", message, err)

		return true
	}

	return false
}

// FatalIfError logs the error and Exit program on Error
func FatalIfError(err error, msgOrPattern string, args ...interface{}) {
	message := fmt.Sprintf(msgOrPattern, args...)

	if LogIfError(err, "%s", message) {
		log.Fatal("FATAL ERROR: Exiting program! - ", message, "\n")
	}
}
