package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"github.com/okochadmytro/tradetracker/internal/config"
)

func main() {
	cmd := flag.String("cmd", "up", "Команда: up | down | version | force <version>")
	steps := flag.Int("steps", 0, "Кількість кроків (0 = всі)")
	forceVersion := flag.Int("force", -1, "Примусово встановити версію міграції")
	flag.Parse()

	// Завантажуємо .env якщо є
	godotenv.Load()
	cfg := config.Load()

	dsn := fmt.Sprintf(
		"mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	// Шлях до файлів міграцій відносно до місця запуску
	migrationsPath := "file://migrations"
	if _, err := os.Stat("migrations"); os.IsNotExist(err) {
		// Якщо запускаємо з cmd/migrate/
		migrationsPath = "file://../../migrations"
	}

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		log.Fatalf("migrate.New: %v", err)
	}
	defer m.Close()

	m.Log = &migLogger{}

	switch *cmd {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
		if err == migrate.ErrNoChange {
			log.Println("No new migrations to apply.")
			return
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-(*steps))
		} else {
			err = m.Down()
		}
		if err == migrate.ErrNoChange {
			log.Println("Nothing to roll back.")
			return
		}
	case "version":
		version, dirty, verErr := m.Version()
		if verErr != nil {
			log.Fatalf("version: %v", verErr)
		}
		log.Printf("Current version: %d (dirty: %v)", version, dirty)
		return
	case "force":
		if *forceVersion < 0 {
			log.Fatal("Вкажи версію: -force <version>")
		}
		err = m.Force(*forceVersion)
	default:
		log.Fatalf("Невідома команда: %s. Доступні: up, down, version, force", *cmd)
	}

	if err != nil {
		log.Fatalf("Migration error: %v", err)
	}
	log.Println("Done.")
}

// migLogger — адаптер для golang-migrate
type migLogger struct{}

func (l *migLogger) Printf(format string, v ...interface{}) {
	log.Printf("[migrate] "+format, v...)
}
func (l *migLogger) Verbose() bool { return true }
