# CapStart Recipe Schema Specification

## Overview

This document defines the complete schema and format for CapStart recipes. Recipes are declarative configurations that automate VM provisioning and setup.

---

## Recipe Structure

```yaml
version: "1.0"
name: "pihole"
title: "Pi-hole"
description: "DNS/DHCP server with ad-blocking"
category: "network"
author: "CapperVM Team"
tags: ["dns", "networking", "ad-blocker"]

# Minimum requirements
requirements:
  cappervm: ">=1.0.0"
  cpu_min: 1
  cpu_recommended: 2
  memory_min: 512  # MB
  memory_recommended: 1024
  disk_min: 5000   # MB
  disk_recommended: 10000

# Virtual machine base configuration
vm:
  os: "ubuntu"
  os_version: "22.04"
  architecture: "x86_64"
  disk_size: 20000  # MB
  cpu: 2
  memory: 1024  # MB

# Network configuration
network:
  interfaces:
    - name: "eth0"
      type: "public"
      required: true
    - name: "eth1"
      type: "private"
      required: false

# User-configurable parameters (shown in UI form)
parameters:
  hostname:
    type: "string"
    label: "Hostname"
    description: "VM hostname"
    default: "pihole"
    required: true
    validation: "^[a-z0-9-]{1,63}$"
    
  admin_password:
    type: "password"
    label: "Admin Password"
    description: "Web interface admin password"
    required: true
    min_length: 8
    
  timezone:
    type: "select"
    label: "Timezone"
    description: "System timezone"
    default: "UTC"
    options:
      - "UTC"
      - "America/New_York"
      - "Europe/London"
      - "Asia/Tokyo"
    required: false
    
  query_logging:
    type: "boolean"
    label: "Enable Query Logging"
    description: "Log all DNS queries"
    default: true
    required: false

# Installation hooks and scripts
installation:
  # Pre-provisioning: before VM creation
  pre_provisioning:
    - name: "validate_config"
      type: "validation"
      script: |
        #!/bin/bash
        # Validate hostname format
        if ! [[ "${HOSTNAME}" =~ ^[a-z0-9-]{1,63}$ ]]; then
          echo "Invalid hostname format"
          exit 1
        fi
  
  # Post-provisioning: after VM boots (runs as root)
  post_provisioning:
    - name: "system_update"
      type: "script"
      timeout: 600  # seconds
      script: |
        #!/bin/bash -e
        apt-get update
        apt-get upgrade -y
        apt-get install -y curl wget
    
    - name: "install_pihole"
      type: "script"
      timeout: 1200
      script: |
        #!/bin/bash -e
        curl -sSL https://install.pi-hole.net | bash /dev/stdin --unattended
        
        # Configure Pi-hole
        echo "ADMIN_PASSWORD=${ADMIN_PASSWORD}" > /etc/pihole/dnsmasq.d/05-pihole-custom.conf
        
        # Restart services
        systemctl restart pihole-FTL.service
    
    - name: "configure_webui"
      type: "script"
      timeout: 300
      script: |
        #!/bin/bash -e
        # Set timezone
        timedatectl set-timezone "${TIMEZONE}"
        
        # Enable query logging if requested
        if [ "${QUERY_LOGGING}" = "true" ]; then
          pihole -q enabled
        fi

# Post-installation hooks (after OS installation for ISO-based installs)
post_install:
  - name: "verify_services"
    type: "health_check"
    timeout: 60
    checks:
      - endpoint: "http://localhost:80"
        method: "GET"
        expected_status: 200
      - endpoint: "dns://127.0.0.1:53"
        query: "google.com"
        expected: "valid_response"

# Environment variables passed to scripts
environment:
  - name: "HOSTNAME"
    source: "parameter"
    parameter: "hostname"
  
  - name: "ADMIN_PASSWORD"
    source: "parameter"
    parameter: "admin_password"
  
  - name: "TIMEZONE"
    source: "parameter"
    parameter: "timezone"
  
  - name: "QUERY_LOGGING"
    source: "parameter"
    parameter: "query_logging"
  
  - name: "VM_ID"
    source: "system"
    
  - name: "VM_IP"
    source: "system"

# Secrets (sensitive values, encrypted at rest)
secrets:
  admin_password:
    source: "parameter"
    parameter: "admin_password"
    encryption: "aes-256"

# Installation method options
install_options:
  - method: "cloud_init"
    description: "Use cloud-init for provisioning"
    default: true
    supported_os: ["ubuntu", "debian", "centos"]
  
  - method: "custom_script"
    description: "Use custom shell scripts"
    default: false
    supported_os: ["ubuntu", "debian", "centos"]

# Dependencies on other recipes or CapperVM features
dependencies:
  recipes: []
  cappervm_features:
    - "networking"
    - "storage"
  
# Output/results after successful installation
outputs:
  admin_url:
    value: "http://${VM_IP}/admin"
    description: "Web interface URL"
    
  default_credentials:
    username: "admin"
    password: "${ADMIN_PASSWORD}"
    note: "Save these credentials in a secure location"
  
  status_check:
    command: "systemctl status pihole-FTL.service"
    description: "Check if Pi-hole is running"

# Rollback configuration
rollback:
  enabled: true
  strategy: "full"  # full, partial, snapshot
  timeout: 300
  
# Testing configuration
testing:
  enabled: true
  tests:
    - name: "web_interface_accessible"
      type: "http"
      url: "http://localhost:80"
      expected_status: 200
    
    - name: "dns_resolution_working"
      type: "dns"
      query: "example.com"
      expected: "valid_response"
    
    - name: "admin_login"
      type: "http"
      method: "POST"
      url: "http://localhost:80/admin/api.php?login"
      body:
        password: "${ADMIN_PASSWORD}"
      expected_status: 200

# Versioning and changelog
version_info:
  current: "1.0.0"
  changelog:
    - version: "1.0.0"
      date: "2026-07-01"
      changes:
        - "Initial release"
        - "Basic Pi-hole installation"
        - "Web UI configuration"

# Community metadata
community:
  author_name: "CapperVM Team"
  author_email: "team@capperdotvm"
  license: "MIT"
  repository: "https://github.com/capperdotvm/capstart-recipes"
  issues_url: "https://github.com/capperdotvm/capstart-recipes/issues"
  documentation_url: "https://docs.capperdotvm/recipes/pihole"
```

---

## Schema Reference

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| version | string | Yes | Schema version (e.g., "1.0") |
| name | string | Yes | Machine-readable name (lowercase, hyphenated) |
| title | string | Yes | Human-readable title |
| description | string | Yes | Short description |
| category | string | Yes | Category (network, compute, storage, etc.) |
| author | string | Yes | Recipe author |
| tags | array | No | Tags for filtering/discovery |

### Requirements

```yaml
requirements:
  cappervm: ">=1.0.0"  # Semantic versioning
  cpu_min: 1
  cpu_recommended: 2
  memory_min: 512      # MB
  memory_recommended: 1024
  disk_min: 5000       # MB
  disk_recommended: 10000
```

### VM Configuration

```yaml
vm:
  os: "ubuntu|debian|centos|windows|alpine"
  os_version: "22.04"
  architecture: "x86_64|arm64"
  disk_size: 20000  # MB
  cpu: 2
  memory: 1024      # MB
```

### Parameters

Each parameter can be:

```yaml
name_of_parameter:
  type: "string|password|number|boolean|select|multiselect|text"
  label: "Display name"
  description: "Help text"
  default: "default_value"
  required: true|false
  
  # String/password validation
  validation: "regex_pattern"  # Optional
  min_length: 8
  max_length: 255
  
  # Numeric validation
  minimum: 1
  maximum: 100
  step: 1
  
  # Select options
  options:
    - "option1"
    - "option2"
  
  # UI hints
  placeholder: "example value"
  help: "Additional help text"
  sensitive: false  # Hide in logs if true
```

### Scripts

```yaml
post_provisioning:
  - name: "script_name"
    type: "script|validation|health_check"
    timeout: 600  # seconds
    retry:
      attempts: 3
      backoff: "exponential"
    script: |
      #!/bin/bash -e
      # Script content
    # OR
    script_file: "path/to/script.sh"
    
    # For validation scripts:
    exit_code: 0  # Expected exit code
    
    # For health checks:
    checks:
      - endpoint: "http://localhost:80"
        method: "GET"
        expected_status: 200
        timeout: 10
```

### Environment Variables

```yaml
environment:
  - name: "VARIABLE_NAME"
    source: "parameter|system|secret|literal"
    parameter: "parameter_name"  # if source=parameter
    value: "literal_value"        # if source=literal
    
    # System sources:
    # - VM_ID: UUID of created VM
    # - VM_IP: Primary IP address
    # - VM_HOSTNAME: Configured hostname
    # - RECIPE_VERSION: Recipe version
```

### Secrets

```yaml
secrets:
  secret_name:
    source: "parameter|generated"
    parameter: "parameter_name"
    encryption: "aes-256"
    rotation_policy: "optional"
```

### Outputs

```yaml
outputs:
  output_name:
    value: "template_string_with_${VARIABLES}"
    description: "What this output is"
    type: "string|url|credentials|command"
```

---

## Parameter Types

### String
Simple text input field
```yaml
type: "string"
validation: "^[a-z0-9-]{1,63}$"  # Regex pattern
min_length: 1
max_length: 255
```

### Password
Masked input, not shown in logs
```yaml
type: "password"
min_length: 8
max_length: 255
```

### Number
Numeric input with optional min/max
```yaml
type: "number"
minimum: 0
maximum: 100
step: 1
```

### Boolean
Checkbox / toggle
```yaml
type: "boolean"
default: true
```

### Select
Dropdown menu, single selection
```yaml
type: "select"
options:
  - "option1"
  - "option2"
  - "option3"
default: "option1"
```

### Multiselect
Dropdown menu, multiple selection
```yaml
type: "multiselect"
options:
  - "option1"
  - "option2"
  - "option3"
default: ["option1"]
```

### Text
Large text area for multi-line input
```yaml
type: "text"
min_length: 1
max_length: 10000
placeholder: "Multi-line text..."
```

---

## Script Execution

### Execution Environment

- **User**: root (for system configuration)
- **Working Directory**: /tmp
- **Shell**: /bin/bash with -e flag (exit on error)
- **Environment**: All defined environment variables available
- **Timeout**: Per-script timeout, default 300s
- **Retry**: Automatic retry on failure (configurable)

### Standard Output

- **Logged**: All stdout captured in execution logs
- **Streamed**: Sent to WebSocket for real-time monitoring
- **Sanitized**: Secrets removed from logs before storage

### Error Handling

```yaml
script: |
  #!/bin/bash -e
  # -e: Exit on error
  # Set -u: Exit on undefined variable
  # Set -o pipefail: Exit on pipe error
  
  if [ $? -ne 0 ]; then
    echo "Error occurred"
    exit 1
  fi
```

---

## Validation Rules

### Required Fields
- `version`, `name`, `title`, `description`, `category`, `author`

### Naming Constraints
- `name`: Lowercase, alphanumeric + hyphens only, 1-63 chars
- `parameter` names: Uppercase (converted), alphanumeric + underscores
- `script` names: Lowercase, alphanumeric + underscores

### Script Validation
- Must start with shebang (`#!/bin/bash`)
- Must not contain hardcoded secrets
- Must complete within timeout
- Exit code 0 = success, non-zero = failure

### Parameter Validation
- All required parameters must be provided
- Parameter values must match validation rules
- Circular dependencies not allowed
- Maximum 50 parameters per recipe

---

## Built-in Recipes

Built-in recipes follow the same schema and live in:
```
/home/megalith/CapperVM/Capper/recipes/
├── pihole/
│   └── recipe.yaml
├── arrsuite/
│   └── recipe.yaml
├── minecraft/
│   └── recipe.yaml
├── homeassistant/
│   └── recipe.yaml
└── jellyfin/
    └── recipe.yaml
```

---

## Examples

### Minimal Recipe
```yaml
version: "1.0"
name: "minimal"
title: "Minimal VM"
description: "Bare minimum VM"
category: "base"
author: "User"

requirements:
  cappervm: ">=1.0.0"
  memory_min: 512

vm:
  os: "ubuntu"
  os_version: "22.04"
  memory: 512

parameters: {}

installation:
  post_provisioning:
    - name: "update"
      type: "script"
      script: |
        #!/bin/bash -e
        apt-get update
        apt-get upgrade -y
```

### Complex Recipe
See full example at top of this document (Pi-hole recipe)

---

## Validation & Error Messages

### Validation Failure Examples

```
ERROR: Recipe 'my-recipe' failed validation:
  - Field 'name' contains invalid characters. Use lowercase and hyphens only.
  - Parameter 'admin_password' has min_length 8 but default is shorter
  - Script 'install_app' references undefined parameter: APP_VERSION
  - Circular dependency detected: recipe A depends on B, B depends on A
```

### Runtime Error Handling

```
ERROR: Recipe execution failed for instance i-12345:
  - Script 'install_app' timed out after 600 seconds
  - Exit code: 124 (timeout)
  - Last log lines:
    [ERROR] Installation hung during download
    [INFO] Downloaded 45% of 2GB package
  
Suggestion: Increase timeout or check internet connectivity
```

---

## Version Compatibility

**Current Schema Version**: 1.0

**Backward Compatibility**: Recipes are version-locked. A recipe built for schema 1.0 will not work with schema 2.0 without migration.

---

**Document Version**: 1.0  
**Created**: 2026-07-01  
**Status**: Ready for Backend Implementation
