package entity

// Wanwu Node Type Definition
const (
	NodeTypeWanWuKnowledgeRetriever NodeType = "WanWuKnowledgeRetriever"
	NodeTypeWanWuFileGenerator      NodeType = "WanWuFileGenerator"
	NodeTypeWanWuFileParser         NodeType = "WanWuFileParser"
	NodeTypeWanWuMultiFileParser    NodeType = "WanWuMultiFileParser"
	NodeTypeWanWuMCPTool            NodeType = "WanWuMCPTool"
	NodeTypeWanWuGUI                NodeType = "WanWuGUI"
	NodeTypeWanWuTool               NodeType = "WanWuTool"
	NodeTypeWanWuQARetriever        NodeType = "WanWuQARetriever"
	NodeTypeWanWuAgent              NodeType = "WanWuAgent"
	NodeTypeWanWuSkill              NodeType = "WanWuSkill"
)

// Wanwu NodeTypeMetas Init
func init() {
	// wanwu新增节点分类
	Categories = append(Categories, Category{
		Key:      "document",
		Name:     "文档",
		EnUSName: "Document",
	})

	// wanwu禁用一些节点
	NodeTypeMetas[NodeTypePlugin].Disabled = true
	NodeTypeMetas[NodeTypeKnowledgeRetriever].Disabled = true
	NodeTypeMetas[NodeTypeQuestionAnswer].Disabled = true
	NodeTypeMetas[NodeTypeKnowledgeIndexer].Disabled = true
	NodeTypeMetas[NodeTypeComment].Disabled = true
	NodeTypeMetas[NodeTypeDatabaseUpdate].Disabled = true
	NodeTypeMetas[NodeTypeDatabaseQuery].Disabled = true
	NodeTypeMetas[NodeTypeDatabaseDelete].Disabled = true
	NodeTypeMetas[NodeTypeDatabaseInsert].Disabled = true
	NodeTypeMetas[NodeTypeVariableAssigner].Disabled = true
	NodeTypeMetas[NodeTypeKnowledgeDeleter].Disabled = true
	// 和前端约定，反序列化节点ID 59 -> 1059
	NodeTypeMetas[NodeTypeJsonDeserialization].ID = 1059

	// wanwu新增节点
	NodeTypeMetas[NodeTypeWanWuTool] = &NodeTypeMeta{
		ID:         1004,
		Key:        NodeTypeWanWuTool,
		DisplayKey: "Tool",
		Name:       "Tool工具",
		Category:   "utilities",
		Desc:       "",
		Color:      "#a5f70cff",
		//IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-Plugin-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "Tool",
		EnUSDescription: "Used to call tools.",
	}

	NodeTypeMetas[NodeTypeWanWuKnowledgeRetriever] = &NodeTypeMeta{
		ID:         1006,
		Key:        NodeTypeWanWuKnowledgeRetriever,
		DisplayKey: "Dataset",
		Name:       "知识库检索",
		Category:   "data",
		Desc:       "在选定的知识中,根据输入变量召回最匹配的信息,并以列表形式返回",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "Knowledge retrieval",
		EnUSDescription: "In the selected knowledge, the best matching information is recalled based on the input variable and returned as an Array.",
	}

	NodeTypeMetas[NodeTypeWanWuFileGenerator] = &NodeTypeMeta{
		ID:         1007,
		Key:        NodeTypeWanWuFileGenerator,
		DisplayKey: "FileGenerator",
		Name:       "文档生成",
		Category:   "document",
		Desc:       "输入文档的内容、格式和文件名，可以生成文档下载链接",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "File generator",
		EnUSDescription: "Generate documents using both the document title and content.",
	}

	NodeTypeMetas[NodeTypeWanWuFileParser] = &NodeTypeMeta{
		ID:         1008,
		Key:        NodeTypeWanWuFileParser,
		DisplayKey: "FileParser",
		Name:       "文档解析",
		Category:   "document",
		Desc:       "输入txt、pdf、docx、xlsx、csv、pptx等格式文档的URL，可以解析提取出文档的文本内容",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "File generator",
		EnUSDescription: "Parse documents content.",
	}

	NodeTypeMetas[NodeTypeWanWuMCPTool] = &NodeTypeMeta{
		ID:         1009,
		Key:        NodeTypeWanWuMCPTool,
		DisplayKey: "MCPTool",
		Name:       "MCP工具",
		Category:   "utilities",
		Desc:       "用于调用MCP服务的工具",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "MCP tool",
		EnUSDescription: "Used to call MCP tools.",
	}

	NodeTypeMetas[NodeTypeWanWuGUI] = &NodeTypeMeta{
		ID:         1010,
		Key:        NodeTypeWanWuGUI,
		DisplayKey: "GUI",
		Name:       "GUI智能体",
		Category:   "utilities",
		Desc:       "通过视觉技术解析用户图形界面上的图像信息，并模拟人类操作行为来执行相应任务，与计算机系统进行交互的智能体。",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-Plugin-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "GUI Agent",
		EnUSDescription: "An intelligent agent that analyzes image information on the user's graphical interface through visual technology and simulates human operational behaviors to perform corresponding tasks, interacting with computer systems.",
	}

	NodeTypeMetas[NodeTypeWanWuMultiFileParser] = &NodeTypeMeta{
		ID:         1011,
		Key:        NodeTypeWanWuMultiFileParser,
		DisplayKey: "MultiFileParser",
		Name:       "多文档解析",
		Category:   "document",
		Desc:       "输入txt、pdf、docx、xlsx、csv、pptx等格式文档的URL，可以解析提取出文档的文本内容，支持多文档解析",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "Multi File generator",
		EnUSDescription: "Parse documents content.",
	}

	NodeTypeMetas[NodeTypeWanWuQARetriever] = &NodeTypeMeta{
		ID:         1012,
		Key:        NodeTypeWanWuQARetriever,
		DisplayKey: "QAset",
		Name:       "问答库检索",
		Category:   "data",
		Desc:       "在选定的问答库中，根据输入变量召回最匹配的信息，并以列表形式返回",
		Color:      "#FF811A",
		// IconURL:      "https://lf3-static.bytednsdoc.com/obj/eden-cn/dvsmryvd_avi_dvsm/ljhwZthlaukjlkulzlp/icon/icon-KnowledgeQuery-v2.jpg",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero: true,
			PostFillNil: true,
		},
		EnUSName:        "Question and answer retrieval",
		EnUSDescription: "In the selected qa pairs, the best matching information is recalled based on the input variable and returned as an Array.",
	}

	NodeTypeMetas[NodeTypeWanWuAgent] = &NodeTypeMeta{
		ID:           1013,
		Key:          NodeTypeWanWuAgent,
		DisplayKey:   "Agent",
		Name:         "智能体",
		Category:     "",
		Desc:         "调用智能体服务执行任务",
		Color:        "#5C62FF",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero:       true,
			PostFillNil:       true,
			IncrementalOutput: true,
			MayUseChatModel:   true,
		},
		EnUSName:        "Agent",
		EnUSDescription: "Call agent service to execute tasks",
	}

	NodeTypeMetas[NodeTypeWanWuSkill] = &NodeTypeMeta{
		ID:           1014,
		Key:          NodeTypeWanWuSkill,
		DisplayKey:   "Skills",
		Name:         "Skills",
		Category:     "utilities",
		Desc:         "调用skill执行任务",
		Color:        "#5C62FF",
		SupportBatch: false,
		ExecutableMeta: ExecutableMeta{
			PreFillZero:       true,
			PostFillNil:       true,
			IncrementalOutput: true,
		},
		EnUSName:        "Skills",
		EnUSDescription: "Call skill to execute tasks",
	}

}
