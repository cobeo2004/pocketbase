package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/search"
	"github.com/pocketbase/pocketbase/tools/types"
)

// bindPgFunctionsApi registers the PostgreSQL functions API endpoints
func bindPgFunctionsApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	sub := rg.Group("/pg-functions")
	sub.GET("", pgFunctionsList).Bind(RequireSuperuserAuth())
	sub.POST("", pgFunctionsCreate).Bind(RequireSuperuserAuth())
	sub.GET("/{id}", pgFunctionsView).Bind(RequireSuperuserAuth())
	sub.PATCH("/{id}", pgFunctionsUpdate).Bind(RequireSuperuserAuth())
	sub.DELETE("/{id}", pgFunctionsDelete).Bind(RequireSuperuserAuth())
	sub.POST("/{id}/invoke", pgFunctionsInvoke).Bind(RequireSuperuserAuth())
	sub.POST("/{id}/validate", pgFunctionsValidate).Bind(RequireSuperuserAuth())
}

type pgFunctionResponse struct {
	*core.PostgreSQLFunction
	Signature string `json:"signature"`
}

func newPgFunctionResponse(function *core.PostgreSQLFunction) *pgFunctionResponse {
	return &pgFunctionResponse{
		PostgreSQLFunction: function,
		Signature:         function.GenerateSignature(),
	}
}

func pgFunctionsList(e *core.RequestEvent) error {
	fieldResolver := search.NewSimpleFieldResolver(
		"id", "created", "updated", "name", "schema", "return_type",
		"language", "volatility", "security", "description", "is_active",
	)

	functions := []*core.PostgreSQLFunction{}

	result, err := search.NewProvider(fieldResolver).
		Query(e.App.DB().Select("*").From(core.FunctionTableName)).
		ParseAndExec(e.Request.URL.Query().Encode(), &functions)

	if err != nil {
		return e.BadRequestError("Failed to fetch functions.", err)
	}

	// Convert to response format
	functionsResponse := make([]*pgFunctionResponse, len(functions))
	for i, function := range functions {
		functionsResponse[i] = newPgFunctionResponse(function)
	}

	// Replace the items in the result
	result.Items = functionsResponse

	return e.JSON(http.StatusOK, result)
}

func pgFunctionsView(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")

	function := &core.PostgreSQLFunction{}
	err := e.App.DB().Select("*").
		From(core.FunctionTableName).
		Where(dbx.HashExp{"id": id}).
		Limit(1).
		One(function)

	if err != nil {
		return e.NotFoundError("Function not found.", err)
	}

	return e.JSON(http.StatusOK, newPgFunctionResponse(function))
}

func pgFunctionsCreate(e *core.RequestEvent) error {
	function := core.NewPostgreSQLFunction()

	form := &pgFunctionForm{app: e.App, function: function}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to read request data.", err)
	}

	if err := form.validate(); err != nil {
		return e.BadRequestError("Validation failed.", err)
	}

	// Apply the form data to the function
	form.applyToFunction()

	// Create the function in PostgreSQL
	if err := createPostgreSQLFunction(e.App, function); err != nil {
		return e.BadRequestError("Failed to create function in database.", err)
	}

	// Save the function metadata
	if err := e.App.DB().Model(function).Insert(); err != nil {
		// Try to rollback the PostgreSQL function creation
		dropPostgreSQLFunction(e.App, function)
		return e.BadRequestError("Failed to save function metadata.", err)
	}

	// Generate migration
	if err := generateFunctionMigration(e.App, function, "create"); err != nil {
		e.App.Logger().Warn("Failed to generate migration for function", "error", err)
	}

	return e.JSON(http.StatusOK, newPgFunctionResponse(function))
}

func pgFunctionsUpdate(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")

	function := &core.PostgreSQLFunction{}
	err := e.App.DB().Select("*").
		From(core.FunctionTableName).
		Where(dbx.HashExp{"id": id}).
		Limit(1).
		One(function)

	if err != nil {
		return e.NotFoundError("Function not found.", err)
	}

	// Keep a copy of the original for rollback
	originalFunction := *function

	form := &pgFunctionForm{app: e.App, function: function}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to read request data.", err)
	}

	if err := form.validate(); err != nil {
		return e.BadRequestError("Validation failed.", err)
	}

	// Apply the form data to the function
	form.applyToFunction()

	// Update the function in PostgreSQL
	if err := updatePostgreSQLFunction(e.App, &originalFunction, function); err != nil {
		return e.BadRequestError("Failed to update function in database.", err)
	}

	// Save the function metadata
	if err := e.App.DB().Model(function).Update(); err != nil {
		// Try to rollback to the original function
		updatePostgreSQLFunction(e.App, function, &originalFunction)
		return e.BadRequestError("Failed to save function metadata.", err)
	}

	// Generate migration
	if err := generateFunctionMigration(e.App, function, "update"); err != nil {
		e.App.Logger().Warn("Failed to generate migration for function", "error", err)
	}

	return e.JSON(http.StatusOK, newPgFunctionResponse(function))
}

func pgFunctionsDelete(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")

	function := &core.PostgreSQLFunction{}
	err := e.App.DB().Select("*").
		From(core.FunctionTableName).
		Where(dbx.HashExp{"id": id}).
		Limit(1).
		One(function)

	if err != nil {
		return e.NotFoundError("Function not found.", err)
	}

	// Drop the function from PostgreSQL
	if err := dropPostgreSQLFunction(e.App, function); err != nil {
		return e.BadRequestError("Failed to drop function from database.", err)
	}

	// Delete the function metadata
	if err := e.App.DB().Model(function).Delete(); err != nil {
		// Try to recreate the function (best effort)
		createPostgreSQLFunction(e.App, function)
		return e.BadRequestError("Failed to delete function metadata.", err)
	}

	// Generate migration
	if err := generateFunctionMigration(e.App, function, "delete"); err != nil {
		e.App.Logger().Warn("Failed to generate migration for function", "error", err)
	}

	return e.NoContent(http.StatusNoContent)
}

func pgFunctionsInvoke(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")

	function := &core.PostgreSQLFunction{}
	err := e.App.DB().Select("*").
		From(core.FunctionTableName).
		Where(dbx.HashExp{"id": id}).
		Limit(1).
		One(function)

	if err != nil {
		return e.NotFoundError("Function not found.", err)
	}

	if !function.IsActive {
		return e.BadRequestError("Function is not active.", nil)
	}

	// Parse function parameters
	var params map[string]interface{}
	if err := e.BindBody(&params); err != nil {
		return e.BadRequestError("Failed to parse parameters.", err)
	}

	// Invoke the function
	result, err := invokePostgreSQLFunction(e.App, function, params)
	if err != nil {
		return e.BadRequestError("Failed to invoke function.", err)
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"result": result,
	})
}

func pgFunctionsValidate(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")

	function := &core.PostgreSQLFunction{}
	err := e.App.DB().Select("*").
		From(core.FunctionTableName).
		Where(dbx.HashExp{"id": id}).
		Limit(1).
		One(function)

	if err != nil {
		return e.NotFoundError("Function not found.", err)
	}

	// Validate the function SQL
	if err := validateFunctionSQL(e.App, function); err != nil {
		return e.JSON(http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
	})
}

// pgFunctionForm handles the function form data
type pgFunctionForm struct {
	app      core.App
	function *core.PostgreSQLFunction

	Name        string                              `json:"name"`
	Schema      string                              `json:"schema"`
	Body        string                              `json:"body"`
	Parameters  []*core.FunctionParameter           `json:"parameters"`
	ReturnType  string                              `json:"returnType"`
	Language    string                              `json:"language"`
	Volatility  string                              `json:"volatility"`
	Security    string                              `json:"security"`
	Description string                              `json:"description"`
	IsActive    bool                                `json:"isActive"`
}

func (form *pgFunctionForm) validate() error {
	// First validate the basic structure
	err := validation.ValidateStruct(form,
		validation.Field(&form.Name, validation.Required, validation.Length(1, 63)),
		validation.Field(&form.Schema, validation.Required, validation.Length(1, 63)),
		validation.Field(&form.Body, validation.Required),
		validation.Field(&form.ReturnType, validation.Required),
		validation.Field(&form.Language, validation.In("plpgsql", "sql", "plpython3u", "plperl")),
		validation.Field(&form.Volatility, validation.In("VOLATILE", "STABLE", "IMMUTABLE")),
		validation.Field(&form.Security, validation.In("DEFINER", "INVOKER")),
		validation.Field(&form.Parameters, validation.By(form.validateParameters)),
	)

	if err != nil {
		return err
	}

	// Create a temporary function for validation
	tempFunction := &core.PostgreSQLFunction{
		Name:       form.Name,
		Schema:     form.Schema,
		Body:       form.Body,
		ReturnType: form.ReturnType,
		Language:   form.Language,
		Parameters: types.JSONArray[*core.FunctionParameter](form.Parameters),
	}

	// Validate the function name
	if err := tempFunction.ValidateName(); err != nil {
		return err
	}

	// Validate the function parameters
	if err := tempFunction.ValidateParameters(); err != nil {
		return err
	}

	// Validate the function body
	if err := tempFunction.ValidateBody(); err != nil {
		return err
	}

	return nil
}

func (form *pgFunctionForm) validateParameters(value interface{}) error {
	params, ok := value.([]*core.FunctionParameter)
	if !ok {
		return fmt.Errorf("invalid parameters format")
	}

	paramNames := make(map[string]bool)
	for i, param := range params {
		if param.Name == "" {
			return fmt.Errorf("parameter %d: name is required", i+1)
		}
		if param.Type == "" {
			return fmt.Errorf("parameter %d: type is required", i+1)
		}
		if paramNames[param.Name] {
			return fmt.Errorf("duplicate parameter name: %s", param.Name)
		}
		paramNames[param.Name] = true

		if param.Mode == "" {
			param.Mode = "IN"
		}
		if param.Mode != "IN" && param.Mode != "OUT" && param.Mode != "INOUT" {
			return fmt.Errorf("parameter %s: invalid mode '%s'", param.Name, param.Mode)
		}
	}

	return nil
}

func (form *pgFunctionForm) applyToFunction() {
	form.function.Name = form.Name
	form.function.Schema = form.Schema
	form.function.Body = form.Body
	form.function.Parameters = types.JSONArray[*core.FunctionParameter](form.Parameters)
	form.function.ReturnType = form.ReturnType
	form.function.Language = form.Language
	form.function.Volatility = form.Volatility
	form.function.Security = form.Security
	form.function.Description = form.Description
	form.function.IsActive = form.IsActive
}

// Helper functions for PostgreSQL operations
func createPostgreSQLFunction(app core.App, function *core.PostgreSQLFunction) error {
	sql := function.GenerateCreateSQL()
	_, err := app.DB().NewQuery(sql).Execute()
	if err != nil {
		return err
	}

	// Clear cached plans after creating/updating function
	_, err = app.DB().NewQuery("DISCARD PLANS").Execute()
	return err
}

func updatePostgreSQLFunction(app core.App, oldFunction, newFunction *core.PostgreSQLFunction) error {
	// Drop the old function and create the new one
	if err := dropPostgreSQLFunction(app, oldFunction); err != nil {
		return err
	}
	return createPostgreSQLFunction(app, newFunction)
}

func dropPostgreSQLFunction(app core.App, function *core.PostgreSQLFunction) error {
	sql := function.GenerateDropSQL()
	_, err := app.DB().NewQuery(sql).Execute()
	if err != nil {
		return err
	}

	// Clear cached plans after dropping function
	_, err = app.DB().NewQuery("DISCARD PLANS").Execute()
	return err
}

func invokePostgreSQLFunction(app core.App, function *core.PostgreSQLFunction, params map[string]interface{}) (interface{}, error) {
	// Build the function call SQL
	var args []string
	for _, param := range function.Parameters {
		if param.Mode == "OUT" {
			continue // Skip OUT parameters
		}

		value, exists := params[param.Name]
		if !exists && param.Default == "" {
			return nil, fmt.Errorf("missing required parameter: %s", param.Name)
		}

		if !exists {
			args = append(args, param.Default)
		} else {
			// Convert value to SQL string representation
			sqlValue, err := convertToSQLValue(value, param.Type)
			if err != nil {
				return nil, fmt.Errorf("invalid value for parameter %s: %w", param.Name, err)
			}
			args = append(args, sqlValue)
		}
	}

	schema := function.Schema
	if schema == "" {
		schema = "public"
	}

	// Clear cached plans to avoid "cached plan must not change result type" errors
	_, err := app.DB().NewQuery("DISCARD PLANS").Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to clear cached plans: %w", err)
	}

	sql := fmt.Sprintf("SELECT %s.%s(%s) as result", schema, function.Name, strings.Join(args, ", "))

	var result interface{}
	err = app.DB().NewQuery(sql).Row(&result)
	return result, err
}

func validateFunctionSQL(app core.App, function *core.PostgreSQLFunction) error {
	// Use RunInTransaction to validate SQL within a transaction that gets rolled back
	var validationErr error
	validationErr = app.RunInTransaction(func(txApp core.App) error {
		sql := function.GenerateCreateSQL()
		_, err := txApp.DB().NewQuery(sql).Execute()
		if err != nil {
			validationErr = err
		}
		// Always return an error to force rollback so function doesn't actually get created
		return fmt.Errorf("validation rollback")
	})

	// If transaction failed for validation reasons, return the validation error
	if validationErr != nil {
		return validationErr
	}

	// If transaction was successful (which means SQL is valid), return nil
	// We can ignore the rollback error since it's intentional
	return nil
}

func convertToSQLValue(value interface{}, paramType string) (string, error) {
	switch v := value.(type) {
	case nil:
		return "NULL", nil
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''")), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format("2006-01-02 15:04:05")), nil
	default:
		// Try to JSON encode for complex types
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("'%s'", strings.ReplaceAll(string(jsonBytes), "'", "''")), nil
	}
}

func generateFunctionMigration(app core.App, function *core.PostgreSQLFunction, operation string) error {
	return core.GenerateFunctionMigration(app, function, operation)
}
