package version

import (
	"fmt"

	"gorm.io/gorm"
)

// Step is one ordered schema repair. The name is the function name and the file name.
type Step struct {
	Name string
	Run  func(db *gorm.DB) error
}

// Steps is the full history, oldest first. Apply runs every step on each migrate.
// A step must no-op when its old shape is already gone.
var Steps = []Step{
	{Name: "V1_EnsureUsersIDIsUUID", Run: V1_EnsureUsersIDIsUUID},
	{Name: "V2_DropUniqueLocationNameIndex", Run: V2_DropUniqueLocationNameIndex},
	{Name: "V3_DropTripGroupID", Run: V3_DropTripGroupID},
	{Name: "V4_AddTripPolicyColumns", Run: V4_AddTripPolicyColumns},
	{Name: "V5_RepairAssignedBranchColumn", Run: V5_RepairAssignedBranchColumn},
}

// Apply runs Steps in order. AutoMigrate follows, and it will not undo these repairs.
func Apply(db *gorm.DB) error {
	for _, step := range Steps {
		if err := step.Run(db); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}
