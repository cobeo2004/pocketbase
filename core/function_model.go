package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase/tools/list"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	FunctionTableName = "_pg_functions"
)

var functionNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PostgreSQLFunction represents a user-defined PostgreSQL function
type PostgreSQLFunction struct {
	BaseModel

	Name        string                 `db:"name" json:"name"`
	Schema      string                 `db:"schema" json:"schema"` // Database schema (default: public)
	Body        string                 `db:"body" json:"body"`     // SQL function body
	Parameters  types.JSONArray[*FunctionParameter] `db:"parameters" json:"parameters"`
	ReturnType  string                 `db:"return_type" json:"returnType"`
	Language    string                 `db:"language" json:"language"`    // Default: plpgsql
	Volatility  string                 `db:"volatility" json:"volatility"` // VOLATILE, STABLE, IMMUTABLE
	Security    string                 `db:"security" json:"security"`    // DEFINER, INVOKER
	Description string                 `db:"description" json:"description"`
	IsActive    bool                   `db:"is_active" json:"isActive"`
}

// FunctionParameter represents a function parameter
type FunctionParameter struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Mode    string `json:"mode"`    // IN, OUT, INOUT
	Default string `json:"default"` // Default value
}

// TableName returns the PostgreSQLFunction model SQL table name.
func (m *PostgreSQLFunction) TableName() string {
	return FunctionTableName
}

// ValidateName validates the function name
func (m *PostgreSQLFunction) ValidateName() error {
	if m.Name == "" {
		return errors.New("function name is required")
	}

	if len(m.Name) > 63 { // PostgreSQL identifier limit
		return errors.New("function name is too long (max 63 characters)")
	}

	if !functionNameRegex.MatchString(m.Name) {
		return errors.New("function name must start with a letter or underscore and contain only letters, numbers, and underscores")
	}

	// Check for reserved words
	reservedWords := []string{
		"select", "insert", "update", "delete", "create", "drop", "alter",
		"table", "index", "view", "function", "procedure", "trigger",
		"user", "role", "grant", "revoke", "commit", "rollback",
	}

	for _, word := range reservedWords {
		if strings.EqualFold(m.Name, word) {
			return fmt.Errorf("function name '%s' is a reserved word", m.Name)
		}
	}

	return nil
}

// ValidateParameters validates function parameters
func (m *PostgreSQLFunction) ValidateParameters() error {
	paramNames := make(map[string]bool)

	for i, param := range m.Parameters {
		if param.Name == "" {
			return fmt.Errorf("parameter %d: name is required", i+1)
		}

		if param.Type == "" {
			return fmt.Errorf("parameter %d: type is required", i+1)
		}

		// Check for duplicate parameter names
		if paramNames[param.Name] {
			return fmt.Errorf("duplicate parameter name: %s", param.Name)
		}
		paramNames[param.Name] = true

		// Validate parameter mode
		if param.Mode != "" && !list.ExistInSlice(param.Mode, []string{"IN", "OUT", "INOUT"}) {
			return fmt.Errorf("parameter %s: invalid mode '%s', must be IN, OUT, or INOUT", param.Name, param.Mode)
		}

		// Default mode is IN
		if param.Mode == "" {
			param.Mode = "IN"
		}
	}

	return nil
}

// ValidateBody validates the function body based on language and return type
func (m *PostgreSQLFunction) ValidateBody() error {
	if m.Body == "" {
		return errors.New("function body is required")
	}

	body := strings.TrimSpace(m.Body)
	language := m.Language
	if language == "" {
		language = "plpgsql"
	}

	switch language {
	case "plpgsql":
		return m.validatePlPgSQLBody(body)
	case "sql":
		return m.validateSQLBody(body)
	}

	// For other languages (plpython3u, plperl, etc.), minimal validation
	return nil
}

// validatePlPgSQLBody validates PL/pgSQL function body
func (m *PostgreSQLFunction) validatePlPgSQLBody(body string) error {
	upper := strings.ToUpper(body)

	// Check if body contains dangerous SQL operations
	dangerousKeywords := []string{
		"DROP TABLE", "DROP DATABASE", "DROP SCHEMA", "TRUNCATE",
		"ALTER SYSTEM", "COPY FROM PROGRAM", "COPY TO PROGRAM",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(upper, keyword) {
			return fmt.Errorf("function body contains potentially dangerous operation: %s", keyword)
		}
	}

	// For non-void functions, check if RETURN statement exists or can be added
	if m.ReturnType != "void" {
		if !strings.HasPrefix(upper, "BEGIN") && !strings.Contains(upper, "RETURN") {
			// Simple expression is OK, will be wrapped with RETURN
			return nil
		}

		if strings.HasPrefix(upper, "BEGIN") && !strings.Contains(upper, "RETURN") {
			return errors.New("PL/pgSQL function with return type must contain RETURN statement")
		}
	}

	return nil
}

// validateSQLBody validates SQL function body
func (m *PostgreSQLFunction) validateSQLBody(body string) error {
	upper := strings.ToUpper(body)

	// SQL functions should typically contain SELECT statements for non-void returns
	if m.ReturnType != "void" {
		if !strings.HasPrefix(upper, "SELECT") && !strings.Contains(body, "SELECT") {
			// Allow simple expressions that will be wrapped with SELECT
			if strings.Contains(upper, "INSERT") || strings.Contains(upper, "UPDATE") ||
			   strings.Contains(upper, "DELETE") || strings.Contains(upper, "CREATE") ||
			   strings.Contains(upper, "DROP") || strings.Contains(upper, "ALTER") {
				return errors.New("SQL function with return type should typically use SELECT statement")
			}
		}
	}

	return nil
}

// GenerateSignature generates the PostgreSQL function signature
func (m *PostgreSQLFunction) GenerateSignature() string {
	var params []string

	for _, param := range m.Parameters {
		paramStr := param.Name + " " + param.Type
		if param.Mode != "IN" {
			paramStr = param.Mode + " " + paramStr
		}
		if param.Default != "" {
			paramStr += " DEFAULT " + param.Default
		}
		params = append(params, paramStr)
	}

	signature := fmt.Sprintf("%s(%s)", m.Name, strings.Join(params, ", "))
	return signature
}

// GenerateCreateSQL generates the CREATE FUNCTION SQL statement
func (m *PostgreSQLFunction) GenerateCreateSQL() string {
	schema := m.Schema
	if schema == "" {
		schema = "public"
	}

	signature := m.GenerateSignature()

	var sql strings.Builder

	// 1. CREATE [OR REPLACE] FUNCTION name(params)
	sql.WriteString(fmt.Sprintf("CREATE OR REPLACE FUNCTION %s.%s", schema, signature))

	// 2. RETURNS rettype
	sql.WriteString(fmt.Sprintf("\nRETURNS %s", m.ReturnType))

	// 3. AS 'definition'
	sql.WriteString("\nAS $$")

	// Handle different function body formats based on language and content
	language := m.Language
	if language == "" {
		language = "plpgsql"
	}

	body := strings.TrimSpace(m.Body)
	if language == "plpgsql" && m.ReturnType != "void" {
		// For plpgsql functions returning a value, ensure proper structure
		if !strings.HasPrefix(strings.ToUpper(body), "BEGIN") {
			// If it's a simple expression, wrap it in BEGIN/END with RETURN
			sql.WriteString("\nBEGIN")
			sql.WriteString("\n    RETURN ")
			sql.WriteString(body)
			if !strings.HasSuffix(body, ";") {
				sql.WriteString(";")
			}
			sql.WriteString("\nEND")
		} else {
			// Body already has BEGIN/END structure
			sql.WriteString("\n")
			sql.WriteString(body)
		}
	} else if language == "plpgsql" && m.ReturnType == "void" {
		// For void functions, ensure proper BEGIN/END structure
		if !strings.HasPrefix(strings.ToUpper(body), "BEGIN") {
			sql.WriteString("\nBEGIN")
			sql.WriteString("\n    ")
			sql.WriteString(body)
			if !strings.HasSuffix(body, ";") {
				sql.WriteString(";")
			}
			sql.WriteString("\nEND")
		} else {
			sql.WriteString("\n")
			sql.WriteString(body)
		}
	} else if language == "sql" {
		// For SQL functions, the body should be a SELECT statement or simple expression
		sql.WriteString("\n")
		if m.ReturnType != "void" && !strings.HasPrefix(strings.ToUpper(body), "SELECT") {
			sql.WriteString("SELECT ")
			sql.WriteString(body)
		} else {
			sql.WriteString(body)
		}
	} else {
		// For other languages (plpython3u, plperl, etc.), use body as-is
		sql.WriteString("\n")
		sql.WriteString(body)
	}

	sql.WriteString("\n$$")

	// 4. LANGUAGE lang_name
	sql.WriteString(fmt.Sprintf("\nLANGUAGE %s", language))

	// 5. [IMMUTABLE|STABLE|VOLATILE]
	if m.Volatility != "" {
		sql.WriteString(fmt.Sprintf("\n%s", m.Volatility))
	}

	// 6. [SECURITY INVOKER|SECURITY DEFINER]
	if m.Security != "" {
		sql.WriteString(fmt.Sprintf("\nSECURITY %s", m.Security))
	}

	// End the statement
	sql.WriteString(";")

	// Add comment if provided
	if m.Description != "" {
		sql.WriteString(fmt.Sprintf("\n\nCOMMENT ON FUNCTION %s.%s IS '%s';",
			schema, signature, strings.ReplaceAll(m.Description, "'", "''")))
	}

	return sql.String()
}

// GenerateDropSQL generates the DROP FUNCTION SQL statement
func (m *PostgreSQLFunction) GenerateDropSQL() string {
	schema := m.Schema
	if schema == "" {
		schema = "public"
	}

	var paramTypes []string
	for _, param := range m.Parameters {
		paramTypes = append(paramTypes, param.Type)
	}

	return fmt.Sprintf("DROP FUNCTION IF EXISTS %s.%s(%s);",
		schema, m.Name, strings.Join(paramTypes, ", "))
}

// NewPostgreSQLFunction creates a new PostgreSQLFunction instance
func NewPostgreSQLFunction() *PostgreSQLFunction {
	return &PostgreSQLFunction{
		BaseModel:   BaseModel{},
		Schema:      "public",
		Language:    "plpgsql",
		Volatility:  "VOLATILE",
		Security:    "INVOKER",
		IsActive:    true,
		Parameters:  types.JSONArray[*FunctionParameter]{},
	}
}
