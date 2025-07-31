package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/tools/inflector"
)

// GenerateFunctionMigration creates a migration file for PostgreSQL function changes
func GenerateFunctionMigration(app App, function *PostgreSQLFunction, operation string) error {
	migrationsDir := filepath.Join(app.DataDir(), "migrations")

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	timestamp := time.Now().Unix()
	functionName := inflector.Snakecase(function.Name)

	var filename string
	var upSQL, downSQL string

	switch operation {
	case "create":
		filename = fmt.Sprintf("%d_create_function_%s.go", timestamp, functionName)
		upSQL = function.GenerateCreateSQL()
		downSQL = function.GenerateDropSQL()

	case "update":
		filename = fmt.Sprintf("%d_update_function_%s.go", timestamp, functionName)
		upSQL = function.GenerateCreateSQL() // CREATE OR REPLACE
		downSQL = fmt.Sprintf("-- Note: This migration updates function %s but does not provide automatic rollback", function.Name)

	case "delete":
		filename = fmt.Sprintf("%d_drop_function_%s.go", timestamp, functionName)
		upSQL = function.GenerateDropSQL()
		downSQL = function.GenerateCreateSQL()

	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}

	migrationContent := generateMigrationFileContent(timestamp, function, operation, upSQL, downSQL)

	filePath := filepath.Join(migrationsDir, filename)
	if err := os.WriteFile(filePath, []byte(migrationContent), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	app.Logger().Info("Generated function migration",
		"file", filename,
		"function", function.Name,
		"operation", operation)

	return nil
}

func generateMigrationFileContent(timestamp int64, function *PostgreSQLFunction, operation, upSQL, downSQL string) string {
	migrationName := fmt.Sprintf("%d_%s_function_%s.go", timestamp, operation, inflector.Snakecase(function.Name))

	var content strings.Builder

	content.WriteString("package migrations\n\n")
	content.WriteString("import (\n")
	content.WriteString("\t\"github.com/pocketbase/dbx\"\n")
	content.WriteString("\t\"github.com/pocketbase/pocketbase/core\"\n")
	content.WriteString(")\n\n")

	content.WriteString(fmt.Sprintf("// %s PostgreSQL function %s\n",
		strings.Title(operation), function.Name))
	content.WriteString("func init() {\n")
	content.WriteString("\tcore.SystemMigrations.Register(func(db dbx.Builder) error {\n")

	// UP migration
	if upSQL != "" {
		content.WriteString("\t\t_, err := db.NewQuery(`\n")
		content.WriteString(fmt.Sprintf("\t\t\t%s\n", strings.ReplaceAll(upSQL, "`", "` + \"`\" + `")))
		content.WriteString("\t\t`).Execute()\n")
		content.WriteString("\t\treturn err\n")
	} else {
		content.WriteString("\t\t// No SQL to execute for this operation\n")
		content.WriteString("\t\treturn nil\n")
	}

	content.WriteString("\t}, func(db dbx.Builder) error {\n")

	// DOWN migration
	if downSQL != "" && !strings.Contains(downSQL, "-- Note:") {
		content.WriteString("\t\t_, err := db.NewQuery(`\n")
		content.WriteString(fmt.Sprintf("\t\t\t%s\n", strings.ReplaceAll(downSQL, "`", "` + \"`\" + `")))
		content.WriteString("\t\t`).Execute()\n")
		content.WriteString("\t\treturn err\n")
	} else {
		content.WriteString("\t\t// Rollback not available for this operation\n")
		if strings.Contains(downSQL, "-- Note:") {
			content.WriteString(fmt.Sprintf("\t\t// %s\n", downSQL))
		}
		content.WriteString("\t\treturn nil\n")
	}

	content.WriteString(fmt.Sprintf("\t}, \"%s\")\n", migrationName))
	content.WriteString("}\n")

	return content.String()
}
