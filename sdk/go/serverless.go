package cappersdk

import "context"

// FunctionsAPI accesses Capper Functions (serverless).
type FunctionsAPI struct{ c *Client }

// Function is a serverless function.
type Function struct {
	ID      string   `json:"id"`
	Project string   `json:"project"`
	Name    string   `json:"name"`
	Runtime string   `json:"runtime"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Status  string   `json:"status"`
}

// InvokeResult is returned from a function invocation.
type InvokeResult struct {
	InvocationID string `json:"invocationId"`
	Status       string `json:"status"`
	DurationMS   int64  `json:"durationMs"`
	Output       string `json:"output"`
	Error        string `json:"error"`
}

// Create creates a function.
func (a *FunctionsAPI) Create(ctx context.Context, fn Function) (Function, error) {
	var out struct {
		Data Function `json:"data"`
	}
	return out.Data, a.c.post(ctx, "functions", fn, &out)
}

// List returns all functions.
func (a *FunctionsAPI) List(ctx context.Context) ([]Function, error) {
	var out struct {
		Data []Function `json:"data"`
	}
	return out.Data, a.c.get(ctx, "functions", &out)
}

// Get returns a function by ID.
func (a *FunctionsAPI) Get(ctx context.Context, id string) (Function, error) {
	var out struct {
		Data Function `json:"data"`
	}
	return out.Data, a.c.get(ctx, "functions/"+id, &out)
}

// Invoke runs a function synchronously with the given payload.
func (a *FunctionsAPI) Invoke(ctx context.Context, id string, payload []byte) (InvokeResult, error) {
	var out struct {
		Data InvokeResult `json:"data"`
	}
	return out.Data, a.c.post(ctx, "functions/"+id+"/invoke", payload, &out)
}

// Delete removes a function.
func (a *FunctionsAPI) Delete(ctx context.Context, id string) error {
	return a.c.del(ctx, "functions/"+id)
}

// MCPAPI accesses Capper MCP servers.
type MCPAPI struct{ c *Client }

// MCPServer is a managed MCP server.
type MCPServer struct {
	ID             string `json:"id"`
	Project        string `json:"project"`
	Name           string `json:"name"`
	Runtime        string `json:"runtime"`
	ApprovalPolicy string `json:"approvalPolicy"`
	Status         string `json:"status"`
}

// MCPCallResult is returned from a tool invocation.
type MCPCallResult struct {
	InvocationID string `json:"invocationId"`
	Decision     string `json:"decision"`
	Status       string `json:"status"`
	ApprovalID   string `json:"approvalId"`
	Result       string `json:"result"`
	Error        string `json:"error"`
}

// CreateServer deploys an MCP server.
func (a *MCPAPI) CreateServer(ctx context.Context, srv MCPServer) (MCPServer, error) {
	var out struct {
		Data MCPServer `json:"data"`
	}
	return out.Data, a.c.post(ctx, "mcp/servers", srv, &out)
}

// ListServers returns all MCP servers.
func (a *MCPAPI) ListServers(ctx context.Context) ([]MCPServer, error) {
	var out struct {
		Data []MCPServer `json:"data"`
	}
	return out.Data, a.c.get(ctx, "mcp/servers", &out)
}

// InvokeTool calls a tool on an MCP server.
func (a *MCPAPI) InvokeTool(ctx context.Context, serverID, tool string, args []byte) (MCPCallResult, error) {
	var out struct {
		Data MCPCallResult `json:"data"`
	}
	return out.Data, a.c.post(ctx, "mcp/servers/"+serverID+"/tools/"+tool+"/invoke", args, &out)
}
