package capstart

import (
	"encoding/json"
	"testing"
)

func TestValidateRecipeRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		recipe      *Recipe
		wantValid   bool
		wantErrors  int
	}{
		{
			name: "valid recipe",
			recipe: &Recipe{
				Name:        "test-app",
				Version:     "1.0.0",
				Title:       "Test App",
				Description: "A test application",
			},
			wantValid: true,
			wantErrors: 0,
		},
		{
			name: "missing name",
			recipe: &Recipe{
				Version:     "1.0.0",
				Title:       "Test App",
				Description: "A test application",
			},
			wantValid: false,
			wantErrors: 1,
		},
		{
			name: "invalid name format",
			recipe: &Recipe{
				Name:        "Test-APP-Invalid",
				Version:     "1.0.0",
				Title:       "Test App",
				Description: "A test application",
			},
			wantValid: false,
			wantErrors: 1,
		},
		{
			name: "invalid version format",
			recipe: &Recipe{
				Name:        "test-app",
				Version:     "invalid",
				Title:       "Test App",
				Description: "A test application",
			},
			wantValid: false,
			wantErrors: 0, // Warning, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRecipe(tt.recipe)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateRecipe() valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if len(result.Errors) != tt.wantErrors {
				t.Errorf("ValidateRecipe() errors = %v, want %v", len(result.Errors), tt.wantErrors)
			}
		})
	}
}

func TestValidateRecipeContent(t *testing.T) {
	validContent := json.RawMessage(`{
		"name": "pihole",
		"title": "Pi-hole",
		"parameters": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"required": true
			}
		},
		"vm": {
			"cpu": 2,
			"memory": 1024
		}
	}`)

	recipe := &Recipe{
		Name:        "pihole",
		Version:     "1.0.0",
		Title:       "Pi-hole",
		Description: "DNS server",
		Content:     validContent,
	}

	result := ValidateRecipe(recipe)
	if !result.Valid {
		t.Errorf("ValidateRecipe() valid = false, want true")
	}
	if result.Metadata.CPUMin == 0 {
		t.Errorf("ValidateRecipe() failed to extract CPU from content")
	}
}

func TestValidateRecipeConfig(t *testing.T) {
	recipeContent := json.RawMessage(`{
		"parameters": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"required": true,
				"min_length": 3,
				"max_length": 63
			},
			"admin_password": {
				"type": "password",
				"label": "Password",
				"required": true,
				"min_length": 8
			},
			"port": {
				"type": "number",
				"label": "Port",
				"minimum": 1024,
				"maximum": 65535
			}
		}
	}`)

	recipe := &Recipe{
		Name:    "test",
		Version: "1.0.0",
		Title:   "Test",
		Content: recipeContent,
	}

	tests := []struct {
		name       string
		config     json.RawMessage
		wantValid  bool
		wantErrors int
	}{
		{
			name: "valid config",
			config: json.RawMessage(`{
				"hostname": "test-host",
				"admin_password": "MyP@ssw0rd",
				"port": 8080
			}`),
			wantValid: true,
			wantErrors: 0,
		},
		{
			name: "missing required parameter",
			config: json.RawMessage(`{
				"admin_password": "MyP@ssw0rd",
				"port": 8080
			}`),
			wantValid: false,
			wantErrors: 1,
		},
		{
			name: "hostname too short",
			config: json.RawMessage(`{
				"hostname": "ab",
				"admin_password": "MyP@ssw0rd",
				"port": 8080
			}`),
			wantValid: false,
			wantErrors: 1,
		},
		{
			name: "password too short",
			config: json.RawMessage(`{
				"hostname": "test-host",
				"admin_password": "short",
				"port": 8080
			}`),
			wantValid: false,
			wantErrors: 1,
		},
		{
			name: "port out of range",
			config: json.RawMessage(`{
				"hostname": "test-host",
				"admin_password": "MyP@ssw0rd",
				"port": 80
			}`),
			wantValid: false,
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRecipeConfig(recipe, tt.config)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateRecipeConfig() valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if len(result.Errors) != tt.wantErrors {
				t.Errorf("ValidateRecipeConfig() errors = %v, want %v", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  - %s: %s", err.Field, err.Message)
				}
			}
		})
	}
}

func TestValidateParameterValue(t *testing.T) {
	tests := []struct {
		name      string
		paramType string
		value     interface{}
		paramDef  map[string]interface{}
		wantValid bool
	}{
		{
			name:      "valid string",
			paramType: "string",
			value:     "test",
			paramDef: map[string]interface{}{
				"min_length": 1.0,
				"max_length": 100.0,
			},
			wantValid: true,
		},
		{
			name:      "string too short",
			paramType: "string",
			value:     "",
			paramDef: map[string]interface{}{
				"min_length": 1.0,
				"max_length": 100.0,
			},
			wantValid: false,
		},
		{
			name:      "valid number",
			paramType: "number",
			value:     50.0,
			paramDef: map[string]interface{}{
				"minimum": 1.0,
				"maximum": 100.0,
			},
			wantValid: true,
		},
		{
			name:      "number out of range",
			paramType: "number",
			value:     150.0,
			paramDef: map[string]interface{}{
				"minimum": 1.0,
				"maximum": 100.0,
			},
			wantValid: false,
		},
		{
			name:      "valid boolean",
			paramType: "boolean",
			value:     true,
			paramDef:  map[string]interface{}{},
			wantValid: true,
		},
		{
			name:      "invalid boolean",
			paramType: "boolean",
			value:     "true",
			paramDef:  map[string]interface{}{},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
			validateParameterValue("test_param", tt.paramType, tt.value, &tt.paramDef, result)
			if result.Valid != tt.wantValid {
				t.Errorf("validateParameterValue() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func BenchmarkValidateRecipe(b *testing.B) {
	content := json.RawMessage(`{
		"name": "test",
		"parameters": {
			"hostname": {"type": "string", "required": true},
			"port": {"type": "number", "minimum": 1, "maximum": 65535}
		}
	}`)

	recipe := &Recipe{
		Name:        "test-app",
		Version:     "1.0.0",
		Title:       "Test App",
		Description: "A test application",
		Content:     content,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateRecipe(recipe)
	}
}
