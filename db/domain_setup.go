package db

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"studio_backend/models"
	"studio_backend/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// migrateDomains runs before the main AutoMigrate. It creates the domains table, seeds the root
// domain (shutterx) and moves any pre-existing rows into it, so the NOT NULL domain_id columns
// added by AutoMigrate afterwards succeed on a database that already holds data.
func migrateDomains(database *gorm.DB) (*models.Domain, error) {
	if err := database.AutoMigrate(&models.Domain{}); err != nil {
		return nil, fmt.Errorf("migrate domains table: %w", err)
	}

	guards := []string{
		// case-insensitive unique domain name
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_name_lower ON domains (lower(name))`,
		// only one root domain (parent_id IS NULL) can exist
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_single_root ON domains ((parent_id IS NULL)) WHERE parent_id IS NULL`,
		// the root is exactly the domain called shutterx; every other domain must have a parent
		fmt.Sprintf(`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_domains_root_name') THEN
				ALTER TABLE domains ADD CONSTRAINT chk_domains_root_name
					CHECK ((parent_id IS NULL) = (name = '%s'));
			END IF;
		END $$`, models.RootDomainName),
	}
	for _, stmt := range guards {
		if err := database.Exec(stmt).Error; err != nil {
			return nil, fmt.Errorf("apply domain constraints: %w", err)
		}
	}

	root, err := seedRootDomain(database)
	if err != nil {
		return nil, err
	}

	if err := moveLegacyRowsToRoot(database, root.ID); err != nil {
		return nil, err
	}
	return root, nil
}

func seedRootDomain(database *gorm.DB) (*models.Domain, error) {
	displayName := "ShutterX"
	var root models.Domain
	err := database.Where("name = ?", models.RootDomainName).
		Attrs(models.Domain{DisplayName: &displayName, Status: models.DomainStatusActive}).
		FirstOrCreate(&root, models.Domain{Name: models.RootDomainName}).Error
	if err != nil {
		return nil, fmt.Errorf("seed root domain: %w", err)
	}
	if !root.IsRoot() {
		return nil, fmt.Errorf("domain %q exists but has a parent; it must be the root", models.RootDomainName)
	}
	return &root, nil
}

// moveLegacyRowsToRoot attaches rows created before domains existed to the root domain.
// It is a no-op on a fresh database and on every run after the first.
func moveLegacyRowsToRoot(database *gorm.DB, rootID uint64) error {
	return database.Transaction(func(tx *gorm.DB) error {
		m := tx.Migrator()

		for _, table := range []string{"users", "roles", "user_devices"} {
			if !m.HasTable(table) {
				continue
			}
			if !m.HasColumn(table, "domain_id") {
				if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN domain_id bigint", table)).Error; err != nil {
					return fmt.Errorf("add %s.domain_id: %w", table, err)
				}
			}
		}

		if m.HasTable("users") {
			if !m.HasColumn("users", "username") {
				if err := tx.Exec(`ALTER TABLE users ADD COLUMN username varchar(50)`).Error; err != nil {
					return fmt.Errorf("add users.username: %w", err)
				}
			}
			if err := tx.Exec(`UPDATE users SET domain_id = ? WHERE domain_id IS NULL`, rootID).Error; err != nil {
				return fmt.Errorf("move users to root domain: %w", err)
			}
			// username = email local part; a later duplicate gets "_<id>" appended
			if err := tx.Exec(`
				UPDATE users u
				SET username = lower(split_part(u.email, '@', 1)) ||
					CASE WHEN EXISTS (
						SELECT 1 FROM users x
						WHERE x.id < u.id
						  AND lower(split_part(x.email, '@', 1)) = lower(split_part(u.email, '@', 1))
					) THEN '_' || u.id ELSE '' END
				WHERE u.username IS NULL OR u.username = ''`).Error; err != nil {
				return fmt.Errorf("backfill usernames: %w", err)
			}
		}

		if m.HasTable("roles") {
			if err := tx.Exec(`UPDATE roles SET domain_id = ? WHERE domain_id IS NULL`, rootID).Error; err != nil {
				return fmt.Errorf("move roles to root domain: %w", err)
			}
		}

		if m.HasTable("user_devices") {
			if m.HasTable("users") {
				if err := tx.Exec(`
					UPDATE user_devices d SET domain_id = u.domain_id
					FROM users u WHERE d.user_id = u.id AND d.domain_id IS NULL`).Error; err != nil {
					return fmt.Errorf("derive user_devices.domain_id: %w", err)
				}
			}
			if err := tx.Exec(`UPDATE user_devices SET domain_id = ? WHERE domain_id IS NULL`, rootID).Error; err != nil {
				return fmt.Errorf("move user_devices to root domain: %w", err)
			}
		}

		// Email and role name were globally unique; they are now unique per domain.
		for _, idx := range []string{"idx_users_email", "idx_roles_name"} {
			if err := tx.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx)).Error; err != nil {
				return fmt.Errorf("drop legacy index %s: %w", idx, err)
			}
		}
		return nil
	})
}

// seedSuperAdmin makes sure superadmin@shutterx exists. It is idempotent: an existing user is never
// modified (its password is left alone). The role is created empty: permissions are never granted to a role automatically.
// The password comes from SUPERADMIN_PASSWORD and is never hardcoded.
func seedSuperAdmin(database *gorm.DB, root *models.Domain) error {
	description := "Full access to the root domain and every child domain"
	var role models.Role
	if err := database.Where("domain_id = ? AND name = ?", root.ID, models.SuperAdminRoleName).
		Attrs(models.Role{Description: &description}).
		FirstOrCreate(&role, models.Role{DomainID: root.ID, Name: models.SuperAdminRoleName}).Error; err != nil {
		return fmt.Errorf("seed superadmin role: %w", err)
	}

	username := models.NormalizeUsername(utils.GetEnv("SUPERADMIN_USERNAME", "superadmin"))
	email := strings.ToLower(utils.GetEnv("SUPERADMIN_EMAIL", username+"@"+models.RootDomainName))

	var user models.User
	err := database.Where("domain_id = ? AND username = ?", root.ID, username).First(&user).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		password := os.Getenv("SUPERADMIN_PASSWORD")
		if len(password) < 8 {
			utils.Logger.Warnw("⚠️ SUPERADMIN_PASSWORD is missing or shorter than 8 characters; superadmin user was not created",
				"login", username+"@"+models.RootDomainName)
			return nil
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash superadmin password: %w", hashErr)
		}
		user = models.User{
			DomainID: root.ID,
			Username: username,
			Name:     "Super Admin",
			Email:    email,
			Password: string(hash),
			Status:   "active",
		}
		if createErr := database.Create(&user).Error; createErr != nil {
			return fmt.Errorf("create superadmin user: %w", createErr)
		}
		utils.Logger.Infow("✅ Superadmin user created", "login", username+"@"+models.RootDomainName)
	case err != nil:
		return fmt.Errorf("look up superadmin user: %w", err)
	}

	if err := database.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
		return fmt.Errorf("assign superadmin role: %w", err)
	}
	return nil
}
