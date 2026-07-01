# CapStart User Guide

**Version**: 1.0  
**Last Updated**: 2026-07-01  

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Using Built-in Recipes](#using-builtin-recipes)
3. [Uploading Custom Recipes](#uploading-custom-recipes)
4. [Managing ISOs](#managing-isos)
5. [Creating VMs from Recipes](#creating-vms-from-recipes)
6. [Monitoring Creation Progress](#monitoring-creation-progress)
7. [Advanced Features](#advanced-features)
8. [Troubleshooting](#troubleshooting)

---

## Getting Started

### Overview

CapStart is a recipe-driven VM provisioning system for CapperVM that allows you to:
- Deploy pre-configured virtual machines in seconds
- Use built-in recipes for common services (PiHole, *arr Suite, Minecraft, etc.)
- Upload custom recipes for your specific needs
- Manage OS installation ISOs
- Track VM creation progress in real-time

### Accessing CapStart

Navigate to **Compute → CapStart** in the CapperVM web interface to access:
- Recipe Browser
- ISO Management
- VM Creation Wizard

---

## Using Built-in Recipes

### Browsing Recipes

1. Navigate to **Recipes** tab
2. View available recipes in a grid layout
3. Filter by:
   - **Category**: Network, Media, Gaming, Smart Home, etc.
   - **Search**: Find recipes by name or description
   - **Type**: Built-in or community recipes

### Recipe Details

Click any recipe card to view:
- **Overview**: Title, author, version, category
- **Description**: What the recipe does
- **Requirements**: CPU, memory, and disk requirements
- **Tags**: Categorization tags
- **Configuration Options**: Parameters you can customize

### Creating a VM from a Recipe

1. Click a recipe to view details
2. Review configuration options
3. Click **"Create VM from Recipe"** to start the wizard
4. Follow the 5-step wizard (see [Creating VMs from Recipes](#creating-vms-from-recipes))

---

## Uploading Custom Recipes

### When to Create Custom Recipes

Create a custom recipe when you want to:
- Automate deployment of your custom applications
- Set up complex multi-service environments
- Standardize VM configurations for your team
- Share configurations with others

### Recipe File Format

Recipes are JSON files with the following structure:

```json
{
  "version": "1.0",
  "name": "my-app",
  "title": "My Custom App",
  "description": "A custom application stack",
  "category": "web",
  "author": "Your Name",
  "tags": ["web", "custom"],
  "requirements": {
    "cpu_min": 2,
    "cpu_recommended": 4,
    "memory_min": 1024,
    "memory_recommended": 2048,
    "disk_min": 20000,
    "disk_recommended": 50000
  },
  "vm": {
    "os": "ubuntu",
    "os_version": "22.04",
    "cpu": 4,
    "memory": 2048,
    "disk_size": 50000
  },
  "parameters": {
    "admin_password": {
      "type": "password",
      "label": "Admin Password",
      "required": true,
      "min_length": 8
    }
  },
  "installation": {
    "post_provisioning": [
      {
        "name": "update_system",
        "type": "script",
        "timeout": 600,
        "script": "#!/bin/bash -e\napt-get update\napt-get upgrade -y"
      }
    ]
  }
}
```

### Uploading a Recipe

1. Click **"Upload Custom Recipe"** button
2. Fill in metadata:
   - **Name**: Machine-readable (lowercase, hyphens)
   - **Title**: Human-readable name
   - **Version**: Semantic versioning (e.g., 1.0.0)
   - **Category**: For organization
   - **Tags**: Comma-separated for discovery
3. Upload the JSON file (drag & drop or click to select)
4. Review and submit
5. Recipe is available immediately

### Recipe Parameter Types

| Type | Usage | Example |
|------|-------|---------|
| `string` | Text input | hostname, domain name |
| `password` | Masked input | admin password, API key |
| `number` | Numeric input | port number, worker count |
| `boolean` | Checkbox | enable feature yes/no |
| `select` | Dropdown | choose version, environment |
| `multiselect` | Multi-check | select multiple options |
| `text` | Large text area | configuration file content |

---

## Managing ISOs

### Uploading ISOs

1. Navigate to **ISOs** tab
2. Drag & drop or click to upload an ISO file
3. System validates and stores the ISO
4. ISO becomes available for OS installation

### ISO Details

For each ISO, you can see:
- OS type (Linux, Windows, etc.)
- Architecture (x86_64, arm64)
- File size
- Verification status
- Upload date

### Verifying ISO Integrity

1. Click **"Verify"** button on an ISO
2. System checks the file integrity
3. Status updates to show verification result
4. Verified ISOs are safe to use for installations

### Deleting ISOs

1. Click **"Delete"** button on an ISO
2. Confirm deletion (cannot be undone)
3. ISO is removed and no longer available

---

## Creating VMs from Recipes

### 5-Step Wizard

The VM creation wizard guides you through:

#### Step 1: Confirm Recipe
- Review recipe details
- Confirm you want to proceed

#### Step 2: Configuration
- Enter parameters specific to the recipe
- Example: hostname, admin password, timezone
- Review requirements and recommendations

#### Step 3: Resources
- Allocate CPU cores (sliders for easy adjustment)
- Allocate memory (MB)
- Allocate disk space (MB)
- Minimum and recommended values shown

#### Step 4: Network
- Set VM hostname
- Configure network settings
- Default network used unless specified

#### Step 5: Review & Deploy
- Review all settings in summary
- Confirm resource allocation
- Click **"Deploy VM"** to create

### Creating a VM from an ISO

1. Navigate to **ISOs** tab
2. Select an ISO
3. Click **"Create VM from ISO"**
4. Configure VM:
   - VM name
   - CPU cores
   - Memory
   - Disk size
   - Network
5. Click **"Boot"** to start installation
6. Complete OS installation in VM console
7. VM ready after installation completes

---

## Monitoring Creation Progress

### Progress Tracking

While a VM is being created:
1. Navigate to **Creation Progress** or from notification
2. See real-time status:
   - **Pending**: Waiting to start
   - **Running**: Currently creating
   - **Success**: VM created successfully
   - **Failed**: Creation failed (see error)

### Execution Logs

- Full logs displayed in the progress view
- Shows each step of the installation
- Useful for troubleshooting failures
- Auto-scrolls to show latest entries

### Safe to Close

- You can safely close the progress page
- Creation continues in the background
- Check VM status in Instances tab
- Progress page updates every 2 seconds

---

## Advanced Features

### Recipe Customization

Create variations of existing recipes:
1. Select a base recipe
2. Choose customizations:
   - Resource requirements
   - Installation hooks
   - Parameters
3. Give custom recipe a name and version
4. Deploy using the customized recipe

### Community Recipes

Browse recipes shared by other users:
1. Navigate to **Community Recipes** tab
2. View ratings, download counts, reviews
3. Filter by category or search
4. Click **"Use Recipe"** to deploy
5. Filter by newest, popular, or highest-rated

### Recipe Versioning

Each recipe can have multiple versions:
- View version history
- Roll back to previous versions
- Compare versions for changes
- Each version is independent

---

## Troubleshooting

### Common Issues

#### Recipe Upload Fails

**Problem**: "Invalid recipe file" error  
**Solution**:
1. Validate JSON syntax (use JSONLint or similar)
2. Ensure all required fields are present
3. Check field types match specification
4. Verify script syntax (bash)

#### VM Creation Fails

**Problem**: "Creation failed" status  
**Solution**:
1. Check error message in progress view
2. Verify recipe requirements vs allocated resources
3. Check available disk space on hypervisor
4. Review execution logs for specific error
5. Retry or contact support

#### ISO Upload Hangs

**Problem**: Upload seems stuck  
**Solution**:
1. Check file size (very large ISOs may take time)
2. Verify internet connection
3. Try uploading a smaller test file first
4. Check browser developer console for errors
5. Retry with different browser

#### VM Starts but Installation Fails

**Problem**: VM created but installation didn't complete  
**Solution**:
1. Access VM console
2. Check for error messages
3. Complete installation manually if needed
4. Check recipe installation scripts for issues
5. Share logs with support team

### Getting Help

If you encounter issues:
1. Check this guide
2. Review execution logs for specific errors
3. Contact your CapperVM administrator
4. Report bugs with detailed logs and steps to reproduce

---

## Best Practices

### Recipe Development

1. **Test locally first**: Test installation scripts before uploading
2. **Document parameters**: Make descriptions clear and helpful
3. **Set realistic requirements**: Over-allocate slightly to avoid failures
4. **Use meaningful names**: Make recipe names descriptive
5. **Version carefully**: Use semantic versioning consistently

### VM Creation

1. **Review settings**: Always review before deploying
2. **Allocate adequately**: Don't skimp on resources
3. **Monitor progress**: Watch initial deployment
4. **Document configuration**: Note custom configurations
5. **Test functionality**: Verify VM works as expected

### Recipe Sharing

1. **Document thoroughly**: Include setup instructions
2. **Use templates**: Start from existing recipes
3. **Get feedback**: Ask for community input
4. **Update regularly**: Fix issues and improve
5. **Respect licensing**: Ensure compliance

---

## FAQ

**Q: Can I use my own ISO for installation?**  
A: Yes! Upload your ISO in the ISO Management tab and use it to create a VM.

**Q: How long does VM creation take?**  
A: Typically 2-10 minutes depending on complexity. Monitor progress in real-time.

**Q: Can I customize a built-in recipe?**  
A: Yes! Use the Recipe Customizer to create variations of built-in recipes.

**Q: What if creation fails?**  
A: Check the error message in logs. Most issues can be resolved by adjusting resources or configuration.

**Q: Can I share my recipes with others?**  
A: Yes! Upload to the community repository so others can use and rate your recipes.

**Q: How do I update a recipe?**  
A: Create a new version with updated content. Old versions remain available.

---

**For more information, visit**: [CapperVM Documentation](/)  
**Report issues**: [GitHub Issues](https://github.com/CapperVM/)  
**Get support**: [Community Forum](/)
