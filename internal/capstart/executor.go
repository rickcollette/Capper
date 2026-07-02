package capstart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// RecipeExecutor executes recipe hooks and scripts
type RecipeExecutor struct {
	recipeStore *RecipeStore
	execStore   *RecipeExecutionStore
}

// NewRecipeExecutor creates a new recipe executor
func NewRecipeExecutor(recipeStore *RecipeStore, execStore *RecipeExecutionStore) *RecipeExecutor {
	return &RecipeExecutor{
		recipeStore: recipeStore,
		execStore:   execStore,
	}
}

// ExecuteRecipe executes a recipe for a VM
func (re *RecipeExecutor) ExecuteRecipe(ctx context.Context, execution *RecipeExecution) error {
	// Fetch recipe
	recipe, err := re.recipeStore.GetRecipe(execution.RecipeID)
	if err != nil {
		execution.Status = "failed"
		execution.ErrorMessage = toPtr(fmt.Sprintf("Recipe not found: %v", err))
		_ = re.execStore.UpdateExecution(execution)
		return err
	}

	// Parse recipe content
	var recipeContent map[string]interface{}
	if err := json.Unmarshal(recipe.Content, &recipeContent); err != nil {
		execution.Status = "failed"
		execution.ErrorMessage = toPtr(fmt.Sprintf("Invalid recipe content: %v", err))
		_ = re.execStore.UpdateExecution(execution)
		return err
	}

	// Merge user config with recipe defaults
	mergedConfig, err := re.mergeConfig(recipe, execution.Config)
	if err != nil {
		execution.Status = "failed"
		execution.ErrorMessage = toPtr(fmt.Sprintf("Config validation failed: %v", err))
		_ = re.execStore.UpdateExecution(execution)
		return err
	}

	// Update execution with merged config
	execution.Config = mergedConfig
	execution.Status = "running"
	execution.StartedAt = timePtr(time.Now())
	if err := re.execStore.UpdateExecution(execution); err != nil {
		return err
	}

	// Execute post_provisioning hooks
	var logs bytes.Buffer
	if installation, ok := recipeContent["installation"].(map[string]interface{}); ok {
		if postProv, ok := installation["post_provisioning"].([]interface{}); ok {
			for _, hookInterface := range postProv {
				if hook, ok := hookInterface.(map[string]interface{}); ok {
					if err := re.executeHook(ctx, &hook, &mergedConfig, &logs); err != nil {
						execution.Status = "failed"
						execution.ErrorMessage = toPtr(err.Error())
						execution.Logs = toPtr(logs.String())
						execution.CompletedAt = timePtr(time.Now())
						_ = re.execStore.UpdateExecution(execution)
						return err
					}
				}
			}
		}
	}

	// Mark as success
	execution.Status = "success"
	execution.Logs = toPtr(logs.String())
	execution.CompletedAt = timePtr(time.Now())
	if err := re.execStore.UpdateExecution(execution); err != nil {
		return err
	}

	return nil
}

// executeHook executes a single installation hook
func (re *RecipeExecutor) executeHook(ctx context.Context, hook *map[string]interface{}, config *json.RawMessage, logs io.StringWriter) error {
	hookType, _ := (*hook)["type"].(string)
	timeout := 300 * time.Second // Default 5 minutes

	if timeoutVal, ok := (*hook)["timeout"].(float64); ok {
		timeout = time.Duration(timeoutVal) * time.Second
	}

	switch hookType {
	case "script":
		return re.executeScript(ctx, hook, config, logs, timeout)
	case "validation":
		return re.executeValidation(ctx, hook, config, logs, timeout)
	case "health_check":
		return re.executeHealthCheck(ctx, hook, config, logs, timeout)
	default:
		return fmt.Errorf("unknown hook type: %s", hookType)
	}
}

// executeScript executes a script hook
func (re *RecipeExecutor) executeScript(ctx context.Context, hook *map[string]interface{}, config *json.RawMessage, logs io.StringWriter, timeout time.Duration) error {
	script, hasScript := (*hook)["script"].(string)
	if !hasScript || script == "" {
		return fmt.Errorf("script content missing")
	}

	// Parse environment variables from config
	env, err := re.buildEnvironment(config)
	if err != nil {
		return err
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute script
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)
	cmd.Env = append(os.Environ(), env...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run script
	if err := cmd.Run(); err != nil {
		errorMsg := fmt.Sprintf("Script failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
		logs.WriteString(errorMsg + "\n")
		return fmt.Errorf("%s", errorMsg)
	}

	// Log output
	logs.WriteString("Script output:\n")
	logs.WriteString(stdout.String())
	if stderr.Len() > 0 {
		logs.WriteString("Stderr:\n")
		logs.WriteString(stderr.String())
	}

	return nil
}

// executeValidation executes a validation hook
func (re *RecipeExecutor) executeValidation(ctx context.Context, hook *map[string]interface{}, config *json.RawMessage, logs io.StringWriter, timeout time.Duration) error {
	// Validation hooks are checks that must pass
	// For now, we'll skip validation hook execution (would be implemented with custom validators)
	logs.WriteString("Validation hook executed (skipped for now)\n")
	return nil
}

// executeHealthCheck executes a health check hook
func (re *RecipeExecutor) executeHealthCheck(ctx context.Context, hook *map[string]interface{}, config *json.RawMessage, logs io.StringWriter, timeout time.Duration) error {
	// Health checks verify that services are running
	// For now, we'll skip health check execution (would be implemented with HTTP checks)
	logs.WriteString("Health check executed (skipped for now)\n")
	return nil
}

// buildEnvironment builds environment variables from config
func (re *RecipeExecutor) buildEnvironment(config *json.RawMessage) ([]string, error) {
	var configMap map[string]interface{}
	if err := json.Unmarshal(*config, &configMap); err != nil {
		return nil, err
	}

	env := []string{}
	for key, value := range configMap {
		envVar := fmt.Sprintf("%s=%v", strings.ToUpper(key), value)
		env = append(env, envVar)
	}

	return env, nil
}

// mergeConfig merges user config with recipe defaults
func (re *RecipeExecutor) mergeConfig(recipe *Recipe, userConfig json.RawMessage) (json.RawMessage, error) {
	// Parse recipe content
	var recipeContent map[string]interface{}
	if err := json.Unmarshal(recipe.Content, &recipeContent); err != nil {
		return nil, err
	}

	// Parse user config
	var userConfigMap map[string]interface{}
	if err := json.Unmarshal(userConfig, &userConfigMap); err != nil {
		return nil, err
	}

	// Extract parameters from recipe
	merged := make(map[string]interface{})

	if parameters, ok := recipeContent["parameters"].(map[string]interface{}); ok {
		for paramName, paramDef := range parameters {
			if paramMap, ok := paramDef.(map[string]interface{}); ok {
				// Use user value if provided, otherwise use default
				if userValue, ok := userConfigMap[paramName]; ok {
					merged[paramName] = userValue
				} else if defaultValue, ok := paramMap["default"]; ok {
					merged[paramName] = defaultValue
				}
			}
		}
	}

	// Add any additional user-provided values
	for key, value := range userConfigMap {
		if _, exists := merged[key]; !exists {
			merged[key] = value
		}
	}

	// Marshal back to JSON
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}

	return mergedJSON, nil
}

// SanitizeLogs removes sensitive information from logs
func SanitizeLogs(logs string) string {
	// Remove passwords
	re := regexp.MustCompile(`(?i)(password|secret|api[_-]?key|token)\s*=\s*[^\s]+`)
	sanitized := re.ReplaceAllString(logs, "$1=***REDACTED***")

	return sanitized
}

// Helper functions
func toPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
