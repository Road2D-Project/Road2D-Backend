package version

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// V5_RepairAssignedBranchColumn undoes a first migrate that put assigned_branch_id
// into the (trip_id, user_id) unique index and marked it NOT NULL.
// AutoMigrate does not narrow that index or drop NOT NULL on its own.
func V5_RepairAssignedBranchColumn(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("trip_members") {
		return nil
	}
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'trip_members'
			  AND column_name = 'assigned_branch_id'
		)
	`).Scan(&exists).Error
	if err != nil {
		return fmt.Errorf("inspect trip_members.assigned_branch_id: %w", err)
	}
	if !exists {
		return nil
	}
	if err := db.Exec(`ALTER TABLE trip_members ALTER COLUMN assigned_branch_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("relax assigned_branch_id: %w", err)
	}
	if err := db.Exec(`
		UPDATE trip_members
		SET assigned_branch_id = NULL
		WHERE assigned_branch_id = '00000000-0000-0000-0000-000000000000'
	`).Error; err != nil {
		return fmt.Errorf("clear nil assigned_branch_id: %w", err)
	}
	var indexDef string
	if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_trip_user'`).Scan(&indexDef).Error; err != nil {
		return fmt.Errorf("inspect idx_trip_user: %w", err)
	}
	if strings.Contains(strings.ToLower(indexDef), "assigned_branch_id") {
		if err := db.Exec(`DROP INDEX IF EXISTS idx_trip_user`).Error; err != nil {
			return fmt.Errorf("drop idx_trip_user: %w", err)
		}
	}
	if err := db.Exec(`
		DO $$
		DECLARE constraint_name text;
		BEGIN
			FOR constraint_name IN
				SELECT c.conname
				FROM pg_constraint c
				JOIN pg_class t ON c.conrelid = t.oid
				JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey)
				WHERE t.relname = 'trip_members'
				  AND a.attname = 'assigned_branch_id'
				  AND c.contype = 'f'
			LOOP
				EXECUTE format('ALTER TABLE trip_members DROP CONSTRAINT IF EXISTS %I', constraint_name);
			END LOOP;
		END $$;
	`).Error; err != nil {
		return fmt.Errorf("drop assigned branch foreign key: %w", err)
	}
	return nil
}
