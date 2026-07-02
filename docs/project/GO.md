# Capper Frontend Implementation - Autonomous Loop (Phase-Based Approach)

**🚀 Ready to begin autonomous implementation**

## Context & References
- PLAN.md: 5-phase implementation roadmap spanning 12-16 weeks
- PASS1.md: Backend API reference (550+ endpoints)
- PASS2.md: Frontend API calls currently implemented (300+)
- PASS3.md: Gap analysis (250+ missing implementations)
- MERMAID/: Workflow diagrams with coverage indicators
- CapperWeb: React + TypeScript frontend codebase

## Current Objective
Systematically implement the frontend features outlined in PLAN.md, prioritized by phase and user impact. Work autonomously on one complete feature per iteration, verifying backend endpoints exist (PASS1.md) and creating frontend implementations following project patterns.

## Phase-by-Phase Workflow

### Phase Selection Logic
1. **Phase 1 (Weeks 1-3)**: Instance Management, ENI, Public IP, Terminal - HIGHEST PRIORITY
2. **Phase 2 (Weeks 4-6)**: VPC Peering, DNS, VPC Endpoints - MEDIUM PRIORITY
3. **Phase 3 (Weeks 7-9)**: S3 Credentials, Bucket Policies - MEDIUM PRIORITY
4. **Phase 4 (Weeks 10-12)**: Placement, Autoscaling, Scheduler - LOWER PRIORITY
5. **Phase 5 (Weeks 13-16)**: CSD Storage, Polish - LOWEST PRIORITY

## Per-Feature Implementation Pattern

For each feature ticket:

### 1️⃣ ANALYZE
- Read feature ticket from PLAN.md (specific line numbers provided)
- Check PASS1.md for backend endpoint definitions
- Review PASS3.md for gap details
- Check relevant MERMAID diagram for workflow context

### 2️⃣ DESIGN
- Identify all required API endpoints
- List React components needed (pages, dialogs, lists, details)
- Map to existing project patterns in CapperWeb
- Identify dependencies on other features

### 3️⃣ IMPLEMENT
- Create API client file (src/api/{feature}.ts) following the pattern in PLAN.md
- Create React components (pages, dialogs, forms) in proper directories
- Implement hooks for data fetching using React Query
- Add error handling and loading states
- Add type definitions for API responses

### 4️⃣ INTEGRATE
- Add routes/navigation entries
- Wire up menu items
- Test against backend API
- Handle edge cases and error scenarios

### 5️⃣ VERIFY
- Confirm all endpoints from ticket are implemented
- Update PASS3.md to mark feature as complete
- Generate summary of what was implemented
- Move to next feature

## Implementation Priorities for This Loop

**Immediate Focus (Phase 1 - Highest Impact):**
1. INSTANCE-001: Instance Reboot Operation
2. INSTANCE-002: Termination Protection Toggle
3. NETWORK-001: ENI Management Page (complete subsystem)
4. NETWORK-002: Public IP Management Page
5. INSTANCE-003: Instance Terminal/Console Access
6. INSTANCE-004: Instance Log Streaming

**Success Criteria:**
- Feature fully functional end-to-end (frontend + backend)
- All required API calls implemented
- Error handling in place
- Type-safe TypeScript implementation
- Follows existing project patterns
- PASS3.md gap for feature updated to "COMPLETED"

## Output Format for Each Feature

After completing each feature:
```
✅ COMPLETED: [TICKET-ID] - [Feature Name]

Backend Endpoints Used:
- [METHOD] /api/v1/...
- [METHOD] /api/v1/...

Frontend Files Created/Modified:
- src/api/{feature}.ts (NEW/MODIFIED)
- src/pages/{section}/{Feature}.tsx (NEW)
- src/components/{Component}.tsx (if needed)

Implementation Summary:
- [Key functionality]
- [User workflows enabled]
- [Complexity level: ⭐-⭐⭐⭐]

Next Feature: [TICKET-ID] - [Feature Name]
```

## Reference Materials Always Available

When analyzing features:
- Check CapperWeb structure for component patterns
- Consult PLAN.md implementation guidelines (lines 650-738)
- Review existing API clients for patterns
- Cross-reference MERMAID diagrams for workflow context
- Use PASS1.md backend reference for endpoint details

## Loop Instructions

- Focus on ONE complete feature per iteration
- Start with Phase 1 (highest impact, 25-35 days estimated)
- Complete all 5 Phase 1 features before moving to Phase 2
- Each feature must be production-ready (not partial implementations)
- Update progress as features complete
- Stop after Phase 1 is complete, or continue autonomously if authorized

---

**Status**: Ready for implementation  
**Created**: 2026-07-01  
**Entry Point**: Phase 1, Feature 1 (INSTANCE-001)
