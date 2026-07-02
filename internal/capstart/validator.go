package capstart

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// ValidateRecipe validates a recipe definition
func ValidateRecipe(recipe *Recipe) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Metadata: RecipeMetadata{},
	}

	// Validate required fields
	if recipe.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: "Recipe name is required",
		})
	}

	if recipe.Title == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "title",
			Message: "Recipe title is required",
		})
	}

	if recipe.Version == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "version",
			Message: "Recipe version is required",
		})
	}

	if recipe.Description == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "description",
			Message: "Recipe description is required",
		})
	}

	// Validate name format (lowercase, alphanumeric + hyphens)
	if recipe.Name != "" && !regexp.MustCompile(`^[a-z0-9-]{1,63}$`).MatchString(recipe.Name) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: "Recipe name must be lowercase, alphanumeric with hyphens, 1-63 characters",
		})
	}

	// Validate version format (semantic versioning)
	if recipe.Version != "" && !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`).MatchString(recipe.Version) {
		result.Valid = false
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "version",
			Message: "Version should follow semantic versioning (e.g., 1.0.0)",
		})
	}

	// Parse and validate recipe content
	if len(recipe.Content) > 0 {
		var content map[string]interface{}
		if err := json.Unmarshal(recipe.Content, &content); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "content",
				Message: fmt.Sprintf("Invalid JSON in recipe content: %v", err),
			})
		} else {
			// Extract metadata from content
			extractMetadata(&content, &result.Metadata)
			validateContent(&content, &result)
		}
	}

	// Parse and validate schema if present
	if len(recipe.Schema) > 0 {
		var schema map[string]interface{}
		if err := json.Unmarshal(recipe.Schema, &schema); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "schema",
				Message: fmt.Sprintf("Invalid JSON in recipe schema: %v", err),
			})
		} else {
			validateSchema(&schema, &result)
		}
	}

	return result
}

// extractMetadata extracts resource requirements from recipe content
func extractMetadata(content *map[string]interface{}, metadata *RecipeMetadata) {
	// Extract VM requirements if present
	if vm, ok := (*content)["vm"].(map[string]interface{}); ok {
		if cpu, ok := vm["cpu"].(float64); ok {
			metadata.CPUMin = int(cpu)
			metadata.CPURecommended = int(cpu)
		}
		if memory, ok := vm["memory"].(float64); ok {
			metadata.MemoryMin = int(memory)
			metadata.MemoryRecommended = int(memory)
		}
		if disk, ok := vm["disk_size"].(float64); ok {
			metadata.DiskMin = int(disk)
			metadata.DiskRecommended = int(disk)
		}
	}

	// Extract requirements if present
	if req, ok := (*content)["requirements"].(map[string]interface{}); ok {
		if cpuMin, ok := req["cpu_min"].(float64); ok {
			metadata.CPUMin = int(cpuMin)
		}
		if cpuRec, ok := req["cpu_recommended"].(float64); ok {
			metadata.CPURecommended = int(cpuRec)
		}
		if memMin, ok := req["memory_min"].(float64); ok {
			metadata.MemoryMin = int(memMin)
		}
		if memRec, ok := req["memory_recommended"].(float64); ok {
			metadata.MemoryRecommended = int(memRec)
		}
		if diskMin, ok := req["disk_min"].(float64); ok {
			metadata.DiskMin = int(diskMin)
		}
		if diskRec, ok := req["disk_recommended"].(float64); ok {
			metadata.DiskRecommended = int(diskRec)
		}
	}
}

// validateContent validates the main recipe content
func validateContent(content *map[string]interface{}, result *ValidationResult) {
	// Validate installation section if present
	if installation, ok := (*content)["installation"].(map[string]interface{}); ok {
		validateInstallation(&installation, result)
	}

	// Validate parameters section if present
	if parameters, ok := (*content)["parameters"].(map[string]interface{}); ok {
		for paramName, paramDef := range parameters {
			validateParameter(paramName, paramDef, result)
		}
	}

	// Validate environment section if present
	if environment, ok := (*content)["environment"].([]interface{}); ok {
		for _, env := range environment {
			if envMap, ok := env.(map[string]interface{}); ok {
				validateEnvironmentVariable(&envMap, result)
			}
		}
	}
}

// validateInstallation validates installation hooks
func validateInstallation(installation *map[string]interface{}, result *ValidationResult) {
	// Validate post_provisioning hooks
	if postProv, ok := (*installation)["post_provisioning"].([]interface{}); ok {
		for i, hook := range postProv {
			if hookMap, ok := hook.(map[string]interface{}); ok {
				validateHook(&hookMap, "post_provisioning", i, result)
			}
		}
	}
}

// validateHook validates a single installation hook
func validateHook(hook *map[string]interface{}, section string, index int, result *ValidationResult) {
	fieldPrefix := fmt.Sprintf("installation.%s[%d]", section, index)

	// Validate required fields
	if name, ok := (*hook)["name"].(string); !ok || name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.name", fieldPrefix),
			Message: "Hook name is required",
		})
		result.Valid = false
	}

	// Validate hook type
	if hookType, ok := (*hook)["type"].(string); ok {
		validTypes := map[string]bool{
			"script":       true,
			"validation":   true,
			"health_check": true,
		}
		if !validTypes[hookType] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.type", fieldPrefix),
				Message: fmt.Sprintf("Invalid hook type: %s (must be script, validation, or health_check)", hookType),
			})
			result.Valid = false
		}
	} else {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.type", fieldPrefix),
			Message: "Hook type is required",
		})
		result.Valid = false
	}

	// Validate script content exists for script-type hooks
	if hookType, ok := (*hook)["type"].(string); ok && hookType == "script" {
		script, hasScript := (*hook)["script"].(string)
		_, hasScriptFile := (*hook)["script_file"].(string)

		if !hasScript && !hasScriptFile {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.script", fieldPrefix),
				Message: "Script or script_file must be provided",
			})
			result.Valid = false
		}

		if hasScript && script == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.script", fieldPrefix),
				Message: "Script content cannot be empty",
			})
			result.Valid = false
		}
	}
}

// validateParameter validates a recipe parameter definition
func validateParameter(name string, paramDef interface{}, result *ValidationResult) {
	if paramMap, ok := paramDef.(map[string]interface{}); ok {
		fieldPrefix := fmt.Sprintf("parameters.%s", name)

		// Validate parameter type
		if paramType, ok := paramMap["type"].(string); ok {
			validTypes := []string{"string", "password", "number", "boolean", "select", "multiselect", "text"}
			isValid := false
			for _, t := range validTypes {
				if t == paramType {
					isValid = true
					break
				}
			}
			if !isValid {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("%s.type", fieldPrefix),
					Message: fmt.Sprintf("Invalid parameter type: %s", paramType),
				})
				result.Valid = false
			}
		}

		// Validate string constraints
		if paramType, ok := paramMap["type"].(string); ok && (paramType == "string" || paramType == "password" || paramType == "text") {
			if minLen, ok := paramMap["min_length"].(float64); ok {
				if maxLen, ok := paramMap["max_length"].(float64); ok {
					if minLen > maxLen {
						result.Warnings = append(result.Warnings, ValidationWarning{
							Field:   fmt.Sprintf("%s.length", fieldPrefix),
							Message: fmt.Sprintf("min_length (%d) is greater than max_length (%d)", int(minLen), int(maxLen)),
						})
					}
				}
			}
		}

		// Validate select options
		if paramType, ok := paramMap["type"].(string); ok && (paramType == "select" || paramType == "multiselect") {
			if options, ok := paramMap["options"].([]interface{}); ok {
				if len(options) == 0 {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:   fmt.Sprintf("%s.options", fieldPrefix),
						Message: "Select/multiselect parameter has no options",
					})
				}
			}
		}
	}
}

// validateEnvironmentVariable validates an environment variable definition
func validateEnvironmentVariable(envVar *map[string]interface{}, result *ValidationResult) {
	if name, ok := (*envVar)["name"].(string); !ok || name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "environment[].name",
			Message: "Environment variable name is required",
		})
		result.Valid = false
	}

	if source, ok := (*envVar)["source"].(string); ok {
		validSources := []string{"parameter", "system", "secret", "literal"}
		isValid := false
		for _, s := range validSources {
			if s == source {
				isValid = true
				break
			}
		}
		if !isValid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "environment[].source",
				Message: fmt.Sprintf("Invalid environment variable source: %s", source),
			})
			result.Valid = false
		}
	}
}

// validateSchema validates parameter schema
func validateSchema(schema *map[string]interface{}, result *ValidationResult) {
	// Basic schema validation
	// More detailed validation would go here based on schema complexity
	if len(*schema) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "schema",
			Message: "Recipe schema is empty",
		})
	}
}

// ValidateRecipeConfig validates user-provided configuration against recipe schema
func ValidateRecipeConfig(recipe *Recipe, config json.RawMessage) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Parse recipe schema
	var recipeContent map[string]interface{}
	if err := json.Unmarshal(recipe.Content, &recipeContent); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "recipe",
			Message: fmt.Sprintf("Failed to parse recipe: %v", err),
		})
		return result
	}

	// Parse user config
	var userConfig map[string]interface{}
	if err := json.Unmarshal(config, &userConfig); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config",
			Message: fmt.Sprintf("Invalid configuration JSON: %v", err),
		})
		return result
	}

	// Extract parameters from recipe
	if parameters, ok := recipeContent["parameters"].(map[string]interface{}); ok {
		// Validate each parameter
		for paramName, paramDef := range parameters {
			if paramMap, ok := paramDef.(map[string]interface{}); ok {
				required, _ := paramMap["required"].(bool)
				paramType, _ := paramMap["type"].(string)

				// Check required parameters
				value, hasValue := userConfig[paramName]
				if required && !hasValue {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   paramName,
						Message: fmt.Sprintf("Required parameter '%s' is missing", paramName),
					})
					continue
				}

				if hasValue {
					// Validate parameter value based on type and constraints
					validateParameterValue(paramName, paramType, value, &paramMap, &result)
				}
			}
		}
	}

	return result
}

// validateParameterValue validates a specific parameter value
func validateParameterValue(name string, paramType string, value interface{}, paramDef *map[string]interface{}, result *ValidationResult) {
	switch paramType {
	case "string", "password", "text":
		if strVal, ok := value.(string); ok {
			// Validate length constraints
			if minLen, ok := (*paramDef)["min_length"].(float64); ok {
				if len(strVal) < int(minLen) {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   name,
						Message: fmt.Sprintf("String length %d is less than minimum %d", len(strVal), int(minLen)),
					})
				}
			}
			if maxLen, ok := (*paramDef)["max_length"].(float64); ok {
				if len(strVal) > int(maxLen) {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   name,
						Message: fmt.Sprintf("String length %d exceeds maximum %d", len(strVal), int(maxLen)),
					})
				}
			}
			// Validate regex pattern if provided
			if pattern, ok := (*paramDef)["validation"].(string); ok {
				if re, err := regexp.Compile(pattern); err == nil {
					if !re.MatchString(strVal) {
						result.Valid = false
						result.Errors = append(result.Errors, ValidationError{
							Field:   name,
							Message: fmt.Sprintf("Value '%s' does not match pattern '%s'", strVal, pattern),
						})
					}
				}
			}
		}

	case "number":
		numVal, ok := value.(float64)
		if !ok {
			// Try to parse as string first
			if strVal, ok := value.(string); ok {
				var err error
				numVal, err = strconv.ParseFloat(strVal, 64)
				if err != nil {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   name,
						Message: fmt.Sprintf("Value '%v' is not a valid number", value),
					})
					return
				}
			} else {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("Value '%v' is not a valid number", value),
				})
				return
			}
		}

		// Check numeric range
		if minimum, ok := (*paramDef)["minimum"].(float64); ok {
			if numVal < minimum {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("Value %v is less than minimum %v", numVal, minimum),
				})
			}
		}
		if maximum, ok := (*paramDef)["maximum"].(float64); ok {
			if numVal > maximum {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("Value %v exceeds maximum %v", numVal, maximum),
				})
			}
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   name,
				Message: fmt.Sprintf("Value '%v' is not a valid boolean", value),
			})
		}

	case "select":
		strVal, ok := value.(string)
		if !ok {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   name,
				Message: fmt.Sprintf("Value '%v' is not a valid string", value),
			})
			return
		}

		if options, ok := (*paramDef)["options"].([]interface{}); ok {
			found := false
			for _, opt := range options {
				if optStr, ok := opt.(string); ok && optStr == strVal {
					found = true
					break
				}
			}
			if !found {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("Value '%s' is not one of the allowed options", strVal),
				})
			}
		}
	}
}
