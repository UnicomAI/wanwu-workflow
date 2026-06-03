package vo

import wanwu_util "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-util"

type WanWuMCPTool struct {
	McpToolInfoList []*WanWuMCPToolInfo `json:"mcpInfoList"`
}

type WanWuMCPToolInfo struct {
	MCPServerURL  string                       `json:"serverUrl"`
	ToolName      string                       `json:"name"`
	Transport     string                       `json:"transport"`     // 传输协议: "sse" 或 "streamable"
	StreamableURL string                       `json:"streamableUrl"` // Streamable HTTP URL
	ApiAuth       wanwu_util.ApiAuthWebRequest `json:"apiAuth"`       // 鉴权信息
	Headers       map[string]string            `json:"headers"`       // 请求头
}

type WanWuGUIParam struct {
	ModelID string `json:"modelId"`
}

type WanWuTool struct {
	ToolID     string `json:"toolId"`
	ApiKey     string `json:"apiKey"`
	ToolType   string `json:"toolType"` // 内置:"builtin" 自定义:"custom"
	ToolName   string `json:"toolName"`
	ActionName string `json:"actionName"`
	ActionID   string `json:"actionId"`
}

type WanWuWorkflow struct {
	WorkflowID string `json:"workflowId"`
}

type WanWuSkill struct {
	SkillType string `json:"skillType"`
	SkillId   string `json:"skillId"`
}

type WanWuMCP struct {
	MCPID       string `json:"mcpId"`
	MCPType     string `json:"mcpType"`
	MCPToolName string `json:"mcpToolName"`
}

type WanWuAgent struct {
	//模型参数复用llm节点
	//知识库参数复用知识库检索节点
	AgentToolParams     []*WanWuTool     `json:"agentToolParams,omitempty"`
	AgentMCPParams      []*WanWuMCP      `json:"agentMCPParams,omitempty"`
	AgentWorkflowParams []*WanWuWorkflow `json:"agentWorkflowParams,omitempty"`
	AgentSkillParams    []*WanWuSkill    `json:"agentSkillParams,omitempty"`
}
