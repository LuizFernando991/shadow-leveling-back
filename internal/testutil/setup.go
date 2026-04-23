package testutil

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/usermetrics"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
	"github.com/LuizFernando991/gym-api/internal/infra/cache"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/infra/http/router"
)

// Setup connects to the test database, resets the schema, runs all migrations,
// and starts a full HTTP test server backed by a NoopSender (emails are
// discarded; tests read verification codes directly from the database).
// Returns the server, the db connection, and a teardown function.
func Setup() (*httptest.Server, *sql.DB, func(), error) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	root := projectRoot()

	if err := godotenv.Overload(filepath.Join(root, ".env.test")); err != nil {
		return nil, nil, nil, fmt.Errorf("load .env.test: %w", err)
	}

	db, err := openDB()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open db: %w", err)
	}

	if err := resetSchema(db); err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("reset schema: %w", err)
	}

	migrationsDir := filepath.Join(root, "internal", "database", "migrations")
	if err := runMigrations(db, migrationsDir); err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	cfg := buildConfig()
	authModule := auth.NewModule(db, cfg.Auth, email.NewNoopSender(), cache.NoopRateLimiter{})
	taskModule := task.NewModule(db)
	userMetricsModule := usermetrics.NewModule(db)
	workoutModule := workout.NewModule(db)
	h := router.NewRouter(cfg, router.Modules{
		Auth:        authModule,
		Task:        taskModule,
		UserMetrics: userMetricsModule,
		Workout:     workoutModule,
	})
	srv := httptest.NewServer(h)

	teardown := func() {
		srv.Close()
		db.Close()
	}

	return srv, db, teardown, nil
}

// Truncate removes all rows from application tables, giving each test a clean
// state without re-running migrations.
func Truncate(db *sql.DB) error {
	_, err := db.Exec(`TRUNCATE TABLE task_completions, tasks, exercise_sets, workout_sessions, workout_exercises, workouts, exercises, email_verifications, sessions, users CASCADE`)
	return err
}

// LatestVerificationCode returns the most recently created verification code
// for the given email and type ("register" or "login"). Tests use this to
// complete the two-step auth flow without a real email provider.
func LatestVerificationCode(db *sql.DB, emailAddr, vtype string) (string, error) {
	var code string
	err := db.QueryRow(
		`SELECT code FROM email_verifications
		 WHERE email = $1 AND type = $2
		 ORDER BY created_at DESC LIMIT 1`,
		emailAddr, vtype,
	).Scan(&code)
	if err != nil {
		return "", fmt.Errorf("get verification code (%s / %s): %w", emailAddr, vtype, err)
	}
	return code, nil
}

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func openDB() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

func resetSchema(db *sql.DB) error {
	_, err := db.Exec(`
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO public;
	`)
	return err
}

func runMigrations(db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec %s: %w", name, err)
		}
	}
	return nil
}

func buildConfig() *config.Config {
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	return &config.Config{
		App: config.AppConfig{
			Env:  "test",
			Name: "gym-test",
		},
		DB: config.DBConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     port,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
			SSLMode:  os.Getenv("DB_SSLMODE"),
		},
		Auth: config.AuthConfig{
			TokenTTL: time.Hour,
		},
	}
}
