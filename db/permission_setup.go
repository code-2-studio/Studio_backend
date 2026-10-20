package db

import (
	"fmt"

	"gorm.io/gorm"
)

// prepareLegacyPermissions turns the old flat permissions table (module_name / permission_name /
// permission_key) into the tree table. It only does so while the table is empty; if rows exist it
// stops, because there is no safe way to guess how they map onto groups, features and permissions.
func prepareLegacyPermissions(database *gorm.DB) error {
	m := database.Migrator()
	if !m.HasTable("permissions") || !m.HasColumn("permissions", "permission_key") {
		return nil
	}

	var rows int64
	if err := database.Table("permissions").Count(&rows).Error; err != nil {
		return fmt.Errorf("count legacy permissions: %w", err)
	}
	if rows > 0 {
		return fmt.Errorf("legacy permissions table has %d rows; map them to feature groups/features/permissions manually before migrating", rows)
	}

	err := database.Exec(`ALTER TABLE permissions
		DROP COLUMN IF EXISTS module_name,
		DROP COLUMN IF EXISTS permission_name,
		DROP COLUMN IF EXISTS permission_key`).Error
	if err != nil {
		return fmt.Errorf("drop legacy permission columns: %w", err)
	}
	return nil
}

// applyPermissionConstraints adds the rules AutoMigrate cannot express. Run after AutoMigrate.
func applyPermissionConstraints(database *gorm.DB) error {
	stmts := []string{
		// a name is unique among siblings within the owning domain (top-level groups share parent 0)
		`DROP INDEX IF EXISTS idx_permissions_sibling_name`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_domain_sibling_name ON permissions (domain_id, COALESCE(parent_id, 0), name)`,
		// valid level, and only groups sit at the top
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_permissions_level') THEN
				ALTER TABLE permissions ADD CONSTRAINT chk_permissions_level
					CHECK (level IN ('group', 'feature', 'permission') AND ((level = 'group') = (parent_id IS NULL)));
			END IF;
		END $$`,
	}
	for _, stmt := range stmts {
		if err := database.Exec(stmt).Error; err != nil {
			return fmt.Errorf("apply permission constraints: %w", err)
		}
	}
	return nil
}
