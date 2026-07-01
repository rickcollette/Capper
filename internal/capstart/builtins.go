package capstart

import (
	"encoding/json"
)

// BuiltinRecipes returns all built-in recipes
func BuiltinRecipes() []*Recipe {
	return []*Recipe{
		PiHoleRecipe(),
		ArrSuiteRecipe(),
		MinecraftRecipe(),
		HomeAssistantRecipe(),
		JellyfinRecipe(),
	}
}

// PiHoleRecipe returns the Pi-hole DNS/DHCP recipe
func PiHoleRecipe() *Recipe {
	schema := json.RawMessage(`{
		"properties": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"description": "VM hostname",
				"default": "pihole",
				"required": true,
				"validation": "^[a-z0-9-]{1,63}$"
			},
			"admin_password": {
				"type": "password",
				"label": "Admin Password",
				"description": "Web interface admin password",
				"required": true,
				"min_length": 8
			},
			"timezone": {
				"type": "select",
				"label": "Timezone",
				"description": "System timezone",
				"default": "UTC",
				"options": ["UTC", "America/New_York", "Europe/London", "Asia/Tokyo"],
				"required": false
			}
		}
	}`)

	content := json.RawMessage(`{
		"version": "1.0",
		"name": "pihole",
		"title": "Pi-hole",
		"description": "DNS/DHCP server with ad-blocking capabilities",
		"category": "network",
		"author": "CapperVM Team",
		"tags": ["dns", "networking", "ad-blocker"],
		"requirements": {
			"cappervm": ">=1.0.0",
			"cpu_min": 1,
			"cpu_recommended": 2,
			"memory_min": 512,
			"memory_recommended": 1024,
			"disk_min": 5000,
			"disk_recommended": 10000
		},
		"vm": {
			"os": "ubuntu",
			"os_version": "22.04",
			"architecture": "x86_64",
			"disk_size": 20000,
			"cpu": 2,
			"memory": 1024
		},
		"installation": {
			"post_provisioning": [
				{
					"name": "system_update",
					"type": "script",
					"timeout": 600,
					"script": "#!/bin/bash -e\\napt-get update\\napt-get upgrade -y\\napt-get install -y curl wget"
				},
				{
					"name": "install_pihole",
					"type": "script",
					"timeout": 1200,
					"script": "#!/bin/bash -e\\ncurl -sSL https://install.pi-hole.net | bash /dev/stdin --unattended"
				}
			]
		},
		"outputs": {
			"admin_url": {
				"value": "http://${VM_IP}/admin",
				"description": "Web interface URL"
			}
		}
	}`)

	return &Recipe{
		ID:          "builtin-pihole-1.0.0",
		Name:        "pihole",
		Version:     "1.0.0",
		Title:       "Pi-hole",
		Description: "DNS/DHCP server with ad-blocking capabilities",
		Category:    "network",
		Author:      "CapperVM Team",
		Tags:        []string{"dns", "networking", "ad-blocker"},
		Schema:      schema,
		Content:     content,
		IsBuiltin:   true,
		IsCommunity: false,
	}
}

// ArrSuiteRecipe returns the *arr suite media management recipe
func ArrSuiteRecipe() *Recipe {
	schema := json.RawMessage(`{
		"properties": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"description": "VM hostname",
				"default": "media-arr",
				"required": true
			},
			"storage_path": {
				"type": "string",
				"label": "Storage Path",
				"description": "Path for media storage",
				"default": "/mnt/media",
				"required": true
			},
			"download_quality": {
				"type": "select",
				"label": "Download Quality",
				"options": ["720p", "1080p", "2160p (4K)"],
				"default": "1080p",
				"required": false
			}
		}
	}`)

	content := json.RawMessage(`{
		"version": "1.0",
		"name": "arrsuite",
		"title": "*arr Suite",
		"description": "Complete media management setup with Sonarr, Radarr, Lidarr, and Prowlarr",
		"category": "media",
		"author": "CapperVM Team",
		"tags": ["media", "sonarr", "radarr", "lidarr", "indexer"],
		"requirements": {
			"cpu_min": 2,
			"cpu_recommended": 4,
			"memory_min": 2048,
			"memory_recommended": 4096,
			"disk_min": 50000,
			"disk_recommended": 100000
		},
		"vm": {
			"os": "ubuntu",
			"os_version": "22.04",
			"cpu": 4,
			"memory": 4096,
			"disk_size": 100000
		},
		"installation": {
			"post_provisioning": [
				{
					"name": "install_arr_suite",
					"type": "script",
					"timeout": 1800,
					"script": "#!/bin/bash -e\\napt-get update && apt-get install -y docker.io docker-compose\\nsystemctl start docker\\nsystemctl enable docker"
				}
			]
		}
	}`)

	return &Recipe{
		ID:          "builtin-arrsuite-1.0.0",
		Name:        "arrsuite",
		Version:     "1.0.0",
		Title:       "*arr Suite",
		Description: "Complete media management setup with Sonarr, Radarr, Lidarr, and Prowlarr",
		Category:    "media",
		Author:      "CapperVM Team",
		Tags:        []string{"media", "sonarr", "radarr", "lidarr", "indexer"},
		Schema:      schema,
		Content:     content,
		IsBuiltin:   true,
		IsCommunity: false,
	}
}

// MinecraftRecipe returns the Minecraft server recipe
func MinecraftRecipe() *Recipe {
	schema := json.RawMessage(`{
		"properties": {
			"server_version": {
				"type": "select",
				"label": "Server Version",
				"options": ["Latest", "1.20.1", "1.19.2"],
				"default": "Latest",
				"required": true
			},
			"max_players": {
				"type": "number",
				"label": "Max Players",
				"minimum": 1,
				"maximum": 100,
				"default": 20,
				"required": false
			},
			"difficulty": {
				"type": "select",
				"label": "Difficulty",
				"options": ["Peaceful", "Easy", "Normal", "Hard"],
				"default": "Normal",
				"required": false
			}
		}
	}`)

	content := json.RawMessage(`{
		"version": "1.0",
		"name": "minecraft",
		"title": "Minecraft Server",
		"description": "Minecraft Java Edition multiplayer server",
		"category": "gaming",
		"author": "CapperVM Team",
		"tags": ["minecraft", "gaming", "server"],
		"requirements": {
			"cpu_min": 2,
			"cpu_recommended": 4,
			"memory_min": 2048,
			"memory_recommended": 4096,
			"disk_min": 20000,
			"disk_recommended": 50000
		},
		"vm": {
			"os": "ubuntu",
			"os_version": "22.04",
			"cpu": 4,
			"memory": 4096,
			"disk_size": 50000
		},
		"installation": {
			"post_provisioning": [
				{
					"name": "install_java",
					"type": "script",
					"timeout": 600,
					"script": "#!/bin/bash -e\\napt-get update\\napt-get install -y openjdk-17-jre-headless"
				}
			]
		}
	}`)

	return &Recipe{
		ID:          "builtin-minecraft-1.0.0",
		Name:        "minecraft",
		Version:     "1.0.0",
		Title:       "Minecraft Server",
		Description: "Minecraft Java Edition multiplayer server",
		Category:    "gaming",
		Author:      "CapperVM Team",
		Tags:        []string{"minecraft", "gaming", "server"},
		Schema:      schema,
		Content:     content,
		IsBuiltin:   true,
		IsCommunity: false,
	}
}

// HomeAssistantRecipe returns the Home Assistant recipe
func HomeAssistantRecipe() *Recipe {
	schema := json.RawMessage(`{
		"properties": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"default": "homeassistant",
				"required": true
			},
			"timezone": {
				"type": "select",
				"label": "Timezone",
				"options": ["UTC", "America/New_York", "Europe/London", "Asia/Tokyo"],
				"default": "UTC",
				"required": false
			}
		}
	}`)

	content := json.RawMessage(`{
		"version": "1.0",
		"name": "homeassistant",
		"title": "Home Assistant",
		"description": "Home automation platform for smart home integration",
		"category": "smarthome",
		"author": "CapperVM Team",
		"tags": ["home-automation", "smarthome", "iot"],
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
		"installation": {
			"post_provisioning": [
				{
					"name": "install_homeassistant",
					"type": "script",
					"timeout": 1200,
					"script": "#!/bin/bash -e\\napt-get update\\napt-get install -y docker.io\\nsystemctl start docker\\nsystemctl enable docker"
				}
			]
		}
	}`)

	return &Recipe{
		ID:          "builtin-homeassistant-1.0.0",
		Name:        "homeassistant",
		Version:     "1.0.0",
		Title:       "Home Assistant",
		Description: "Home automation platform for smart home integration",
		Category:    "smarthome",
		Author:      "CapperVM Team",
		Tags:        []string{"home-automation", "smarthome", "iot"},
		Schema:      schema,
		Content:     content,
		IsBuiltin:   true,
		IsCommunity: false,
	}
}

// JellyfinRecipe returns the Jellyfin media server recipe
func JellyfinRecipe() *Recipe {
	schema := json.RawMessage(`{
		"properties": {
			"hostname": {
				"type": "string",
				"label": "Hostname",
				"default": "jellyfin",
				"required": true
			},
			"media_path": {
				"type": "string",
				"label": "Media Library Path",
				"default": "/mnt/media",
				"required": true
			}
		}
	}`)

	content := json.RawMessage(`{
		"version": "1.0",
		"name": "jellyfin",
		"title": "Jellyfin",
		"description": "Free media system for organizing and sharing media across devices",
		"category": "media",
		"author": "CapperVM Team",
		"tags": ["media-server", "streaming", "movies", "tv"],
		"requirements": {
			"cpu_min": 2,
			"cpu_recommended": 4,
			"memory_min": 1024,
			"memory_recommended": 2048,
			"disk_min": 50000,
			"disk_recommended": 100000
		},
		"vm": {
			"os": "ubuntu",
			"os_version": "22.04",
			"cpu": 4,
			"memory": 2048,
			"disk_size": 100000
		},
		"installation": {
			"post_provisioning": [
				{
					"name": "install_jellyfin",
					"type": "script",
					"timeout": 900,
					"script": "#!/bin/bash -e\\napt-get update\\napt-get install -y curl gnupg\\ncurl https://repo.jellyfin.org/debian/jellyfin_team.gpg.key | apt-key add -"
				}
			]
		}
	}`)

	return &Recipe{
		ID:          "builtin-jellyfin-1.0.0",
		Name:        "jellyfin",
		Version:     "1.0.0",
		Title:       "Jellyfin",
		Description: "Free media system for organizing and sharing media across devices",
		Category:    "media",
		Author:      "CapperVM Team",
		Tags:        []string{"media-server", "streaming", "movies", "tv"},
		Schema:      schema,
		Content:     content,
		IsBuiltin:   true,
		IsCommunity: false,
	}
}

// LoadBuiltinRecipes loads all built-in recipes into the database
func LoadBuiltinRecipes(store *RecipeStore) error {
	recipes := BuiltinRecipes()
	for _, recipe := range recipes {
		// Check if already exists
		existing, err := store.GetRecipeByName(recipe.Name, recipe.Version)
		if err == nil && existing != nil {
			// Recipe already exists, skip
			continue
		}

		// Create new recipe
		if err := store.CreateRecipe(recipe); err != nil {
			return err
		}
	}

	return nil
}
