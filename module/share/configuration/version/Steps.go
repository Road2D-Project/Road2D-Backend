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
	{Name: "V6_AddUserNumberPhone", Run: V6_AddUserNumberPhone},
	{Name: "V7_AddImageURLColumns", Run: V7_AddImageURLColumns},
	{Name: "V8_AddTripIdempotencyKey", Run: V8_AddTripIdempotencyKey},
	{Name: "V9_AddTripBranchMetadata", Run: V9_AddTripBranchMetadata},
	{Name: "V10_AddUserSessionVersion", Run: V10_AddUserSessionVersion},
	{Name: "V11_AddDestinationCreator", Run: V11_AddDestinationCreator},
}

// Apply runs Steps in order. AutoMigrate follows, and it will not undo these repairs.
func Apply(db *gorm.DB) error {
	return applySteps(db, Steps)
}

// ApplyAfterModels repairs constraints/backfills that require newly created tables.
// These steps are idempotent, like the legacy repairs, and run on every migrate.
func ApplyAfterModels(db *gorm.DB) error {
	return applySteps(db, []Step{
		{Name: "V12_AddChatFoundation", Run: V12_AddChatFoundation},
		{Name: "V13_AddOutboxLifecycle", Run: V13_AddOutboxLifecycle},
	})
}

func applySteps(db *gorm.DB, steps []Step) error {
	for _, step := range steps {
		if err := step.Run(db); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}
