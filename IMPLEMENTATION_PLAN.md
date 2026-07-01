# CapStart Integration Implementation Plan

## Executive Summary

Integrate CapStart as a core foundation for CapperVM, enabling users to:
1. Use CapStart recipes to create pre-configured VMs
2. Upload custom OS installation ISOs
3. Perform complete VM installations with minimal configuration
4. Leverage community recipes (PiHole, *arr suite, Minecraft, etc.)

---

## Phase 1: Foundation & Architecture (Weeks 1-2)

### 1.1 CapStart Integration Architecture

**Objective**: Design the integration between CapperVM and CapStart

**Tasks**:
- [ ] Analyze CapStart codebase and API
- [ ] Define recipe schema and structure
- [ ] Create recipe validation framework
- [ ] Design VM provisioning workflow
- [ ] Document integration points

**Deliverables**:
- Architecture documentation
- Recipe schema spec
- Integration guidelines

**Files**:
- `/home/megalith/CapperVM/Capper/CAPSTART_ARCHITECTURE.md`
- `/home/megalith/CapperVM/Capper/RECIPE_SCHEMA.md`

---

### 1.2 Backend CapStart API Endpoints

**Objective**: Create backend endpoints for recipe management and VM creation

**API Endpoints to Implement**:

```
# Recipe Management
GET    /api/v1/capstart/recipes                    - List all recipes
POST   /api/v1/capstart/recipes                    - Upload/register recipe
GET    /api/v1/capstart/recipes/{recipeId}        - Get recipe details
DELETE /api/v1/capstart/recipes/{recipeId}        - Delete recipe
PUT    /api/v1/capstart/recipes/{recipeId}        - Update recipe metadata

# Recipe Execution
POST   /api/v1/capstart/recipes/{recipeId}/create-vm    - Create VM from recipe
GET    /api/v1/capstart/recipes/{recipeId}/validate     - Validate recipe
POST   /api/v1/capstart/recipes/{recipeId}/test         - Test recipe

# Built-in Recipes
GET    /api/v1/capstart/recipes/builtin                 - List built-in recipes
GET    /api/v1/capstart/recipes/builtin/pihole          - PiHole recipe
GET    /api/v1/capstart/recipes/builtin/arrsuite        - *arr suite recipe
GET    /api/v1/capstart/recipes/builtin/minecraft       - Minecraft recipe

# ISO Management
GET    /api/v1/capstart/isos                            - List uploaded ISOs
POST   /api/v1/capstart/isos                            - Upload ISO
DELETE /api/v1/capstart/isos/{isoId}                    - Delete ISO
POST   /api/v1/capstart/isos/{isoId}/verify             - Verify ISO integrity

# VM Installation
POST   /api/v1/capstart/install                         - Start OS installation
GET    /api/v1/capstart/install/{jobId}                 - Get installation status
POST   /api/v1/capstart/install/{jobId}/cancel          - Cancel installation
```

**Implementation Steps**:
1. Create recipe CRUD endpoints
2. Create ISO management endpoints
3. Create VM creation workflow endpoints
4. Add recipe validation logic
5. Add error handling and status tracking

**Complexity**: ⭐⭐⭐ High

---

## Phase 2: Recipe System (Weeks 3-4)

### 2.1 Recipe Storage & Validation

**Objective**: Implement recipe storage, validation, and versioning

**Tasks**:
- [ ] Create recipe database schema
- [ ] Implement recipe file storage (S3/local)
- [ ] Create recipe parser/validator
- [ ] Implement recipe versioning
- [ ] Add recipe dependency resolution

**Files**:
- Backend recipe service handlers
- Recipe validation logic
- Database migrations

---

### 2.2 Built-in Recipe Library

**Objective**: Create built-in recipes for common use cases

**Built-in Recipes to Create**:

1. **PiHole Recipe**
   - DNS/DHCP server setup
   - Web interface configuration
   - Ad-blocking lists
   - Backup/restore procedures

2. ***arr Suite Recipe**
   - Sonarr (TV show management)
   - Radarr (Movie management)
   - Lidarr (Music management)
   - Prowlarr (Indexer management)
   - Shared storage configuration

3. **Minecraft Server Recipe**
   - Server version selection
   - Java/Kotlin setup
   - World generation
   - Mod support
   - Performance tuning

4. **Additional Recipes**:
   - Home Assistant
   - Jellyfin/Plex
   - GitLab/Gitea
   - Nextcloud
   - OpenVPN

**Files**:
- `/home/megalith/CapStart/recipes/pihole/`
- `/home/megalith/CapStart/recipes/arrsuite/`
- `/home/megalith/CapStart/recipes/minecraft/`
- etc.

**Complexity**: ⭐⭐ Medium

---

## Phase 3: Frontend Recipe Management (Weeks 5-6)

### 3.1 Recipe Browser & Details UI

**Objective**: Create frontend for browsing and managing recipes

**Components to Create**:

1. **RecipeBrowser.tsx**
   - List all available recipes
   - Search/filter by category
   - View recipe details
   - See requirements and preview

2. **RecipeDetail.tsx**
   - Full recipe information
   - Configuration options
   - Preview generated VM spec
   - Create VM button

3. **RecipeUpload.tsx**
   - Upload custom recipe
   - Recipe validation feedback
   - File preview
   - Metadata configuration

4. **RecipeLibrary.tsx**
   - Browse built-in recipes
   - Filter by category
   - View documentation
   - Quick create buttons

**API Clients to Create**:
- `src/api/capstart-recipes.ts` - Recipe management
- `src/api/capstart-isos.ts` - ISO management

**Files**:
- `src/pages/capstart/RecipeBrowser.tsx`
- `src/pages/capstart/RecipeDetail.tsx`
- `src/pages/capstart/RecipeUpload.tsx`
- `src/pages/capstart/RecipeLibrary.tsx`
- `src/api/capstart-recipes.ts`
- `src/api/capstart-isos.ts`

**Complexity**: ⭐⭐ Medium

---

### 3.2 ISO Upload & Management UI

**Objective**: Create UI for uploading and managing OS installation ISOs

**Components to Create**:

1. **ISOUpload.tsx**
   - Drag-and-drop ISO upload
   - Progress tracking
   - File validation
   - Metadata configuration

2. **ISOManagement.tsx**
   - List uploaded ISOs
   - View details
   - Delete ISOs
   - Verify integrity

3. **OSInstaller.tsx**
   - Select ISO to install
   - Configure installation parameters
   - Monitor installation progress
   - Handle installation errors

**Files**:
- `src/pages/capstart/ISOUpload.tsx`
- `src/pages/capstart/ISOManagement.tsx`
- `src/pages/capstart/OSInstaller.tsx`

**Complexity**: ⭐⭐⭐ High (file upload, progress tracking)

---

## Phase 4: VM Creation Workflow (Weeks 7-8)

### 4.1 Recipe-Based VM Creation

**Objective**: Implement full workflow to create VMs from recipes

**Workflow Steps**:
1. User selects recipe
2. System presents configuration options
3. User customizes settings (resources, hostname, etc.)
4. System generates VM spec
5. Backend creates VM with recipe execution
6. Frontend monitors creation progress
7. VM ready for use

**Components to Create**:

1. **RecipeVMWizard.tsx**
   - Multi-step wizard
   - Recipe selection
   - Configuration form
   - Resource allocation
   - Network settings
   - Review & create

2. **CreationProgress.tsx**
   - Real-time progress updates
   - Log streaming
   - Error handling
   - Retry options

**Implementation**:
- Create wizard state management
- Build configuration form dynamically from recipe schema
- Implement progress polling
- Add error recovery

**Complexity**: ⭐⭐⭐ High

---

### 4.2 ISO Installation Workflow

**Objective**: Implement VM creation with OS installation from ISO

**Workflow Steps**:
1. User selects ISO
2. System creates base VM
3. System boots VM with ISO
4. User configures installation (keyboard, disk, etc.)
5. System monitors installation progress
6. Installation completes, VM ready
7. Post-installation configuration (optional)

**Implementation**:
- ISO mounting and boot configuration
- Installation progress tracking
- Network-based kickstart/preseed support
- Post-install hook execution

**Complexity**: ⭐⭐⭐⭐ Very High

---

## Phase 5: Advanced Features (Weeks 9-10)

### 5.1 Recipe Customization & Templates

**Objective**: Allow users to create custom recipes from templates

**Features**:
- Recipe builder UI
- Template inheritance
- Variable interpolation
- Secret management
- Configuration validation

**Files**:
- `src/pages/capstart/RecipeBuilder.tsx`
- Backend recipe templating system

---

### 5.2 Community Recipe Repository

**Objective**: Enable sharing and discovery of user-created recipes

**Features**:
- Recipe publishing (with user permission)
- Community ratings/reviews
- Version management
- Documentation hosting
- Dependency tracking

**Implementation**:
- Create recipe marketplace backend
- Build community submission workflow
- Add recipe discovery/search
- Implement quality standards

---

### 5.3 Recipe Automation & Scheduling

**Objective**: Enable automated VM creation based on schedules

**Features**:
- Scheduled recipe execution
- Batch VM creation
- Resource-based scaling
- Notification on completion

---

## Phase 6: Testing & Documentation (Weeks 11-12)

### 6.1 Comprehensive Testing

**Test Coverage**:
- [ ] Unit tests for recipe validation
- [ ] Integration tests for recipe execution
- [ ] E2E tests for complete workflows
- [ ] Performance tests for large ISOs
- [ ] Error recovery tests

---

### 6.2 Documentation

**Documentation to Create**:
- [ ] Recipe format specification
- [ ] Recipe development guide
- [ ] User guide for built-in recipes
- [ ] Custom recipe creation tutorial
- [ ] API documentation
- [ ] Video tutorials

---

## Implementation Resources

### Technology Stack
- **Backend**: Go (existing)
- **Frontend**: React + TypeScript (existing)
- **Database**: PostgreSQL (existing)
- **File Storage**: S3 or local filesystem
- **Task Queue**: For long-running operations
- **Monitoring**: Prometheus/Grafana

### Team Requirements
- 2-3 backend engineers
- 2 frontend engineers
- 1 DevOps engineer
- 1 QA engineer
- 1 Technical writer

### Timeline
- **Total Duration**: 12 weeks
- **Critical Path**: Phases 1-3 must complete before Phase 4
- **Parallel Work**: UI development (Phase 3) can start after Phase 1 design

---

## Success Criteria

### Phase 1
- ✅ Architecture documented and approved
- ✅ Recipe schema finalized
- ✅ All API endpoints implemented

### Phase 2
- ✅ Recipe storage working
- ✅ Validation logic robust
- ✅ Built-in recipes tested

### Phase 3
- ✅ Recipe UI components complete
- ✅ ISO management working
- ✅ File uploads reliable

### Phase 4
- ✅ Recipe-based VM creation working
- ✅ ISO installation functional
- ✅ Progress tracking accurate

### Phase 5
- ✅ Advanced features working
- ✅ Community features enabled
- ✅ Automation reliable

### Phase 6
- ✅ Test coverage >80%
- ✅ Documentation complete
- ✅ User feedback positive

---

## Risk Mitigation

### High-Risk Items

1. **ISO Installation Complexity**
   - Risk: OS installation is complex with many variables
   - Mitigation: Start with Linux (Debian-based), add Windows later
   - Fallback: Provide pre-configured images instead of ISO installation

2. **Recipe Compatibility**
   - Risk: Recipes might not work across different environments
   - Mitigation: Extensive testing, clear documentation, version constraints
   - Fallback: Recipe validation warnings

3. **Performance with Large ISOs**
   - Risk: Large ISO files (>5GB) could impact performance
   - Mitigation: Streaming uploads, background processing
   - Fallback: ISO URL-based download support

4. **Security of User Recipes**
   - Risk: User-provided recipes could contain malicious code
   - Mitigation: Sandboxed execution, code review, signed recipes
   - Fallback: Admin approval for community recipes

---

## Next Steps

1. **Schedule kickoff meeting** with stakeholders
2. **Review architecture** with backend team
3. **Create detailed recipe schema** specification
4. **Begin Phase 1 work** on API design
5. **Start CapStart integration** analysis
6. **Set up development environment** for testing

---

**Document Version**: 1.0  
**Created**: 2026-07-01  
**Status**: Ready for Implementation Planning  
**Approval Required**: Yes
