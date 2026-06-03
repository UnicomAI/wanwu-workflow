package adaptor

import (
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/knowledge"
	wanwu_agent "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-agent"
	wanwu_filegenerator "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-filegenerator"
	wanwu_fileparser "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-fileparser"
	wanwu_gui "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-gui"
	wanwu_mcp "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-mcp"
	wanwu_qa "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-qa"
	wanwu_skill "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-skill"
	wanwu_tool "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-tool"
)

// RegisterWanwuAllNodeAdaptors 参考RegisterAllNodeAdaptors
func RegisterWanwuAllNodeAdaptors() {
	// register a generator function so that each time a NodeAdaptor is needed,
	// we can provide a brand new Config instance.
	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuKnowledgeRetriever, func() nodes.NodeAdaptor {
		return &knowledge.WanWuRetrieveConfig{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuFileParser, func() nodes.NodeAdaptor {
		return &wanwu_fileparser.WanWuRetrieveConfig{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuFileGenerator, func() nodes.NodeAdaptor {
		return &wanwu_filegenerator.WanWuRetrieveConfig{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuMCPTool, func() nodes.NodeAdaptor {
		return &wanwu_mcp.Config{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuGUI, func() nodes.NodeAdaptor {
		return &wanwu_gui.Config{}
	})
	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuTool, func() nodes.NodeAdaptor {
		return &wanwu_tool.Config{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuMultiFileParser, func() nodes.NodeAdaptor {
		return &wanwu_fileparser.WanWuMultiFileParserConfig{}
	})
	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuQARetriever, func() nodes.NodeAdaptor {
		return &wanwu_qa.WanWuQARetrieveConfig{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuAgent, func() nodes.NodeAdaptor {
		return &wanwu_agent.Config{}
	})

	nodes.RegisterNodeAdaptor(entity.NodeTypeWanWuSkill, func() nodes.NodeAdaptor {
		return &wanwu_skill.Config{}
	})
}
