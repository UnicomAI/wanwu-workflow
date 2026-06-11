package router

import (
	"github.com/cloudwego/hertz/pkg/app"
	hertz_server "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/coze-dev/coze-studio/backend/api/handler/coze"

	wanwu_mock "github.com/UnicomAI/wanwu-workflow/wanwu-backend/internal/server/http/handler/wanwu-mock"
)

// ！！！同步于：backend/api/router/coze/api.go
// ！！！同步于：backend/api/router/coze/api.go
// ！！！同步于：backend/api/router/coze/api.go

func Register(r *hertz_server.Hertz) {
	// TODO auto generated

	// FIXME 这里实际对应的是 ./configs/static/api/static 而非 ./configs/static ??
	r.Static("/api/static", "./configs/static")

	root := r.Group("/", rootMw()...)
	{
		_api := root.Group("/api", _apiMw()...)
		{
			_bot := _api.Group("/bot", _botMw()...)
			_bot.POST("/get_type_list", append(_gettypelistMw(), coze.GetTypeList)...)
			_bot.POST("/upload_file", append(_uploadfileMw(), coze.UploadFile)...)
			_bot.POST("/upload_file_by_wanwu", append(_uploadfileByBse64Mw(), coze.UploadFileByWanwu)...)
		}
		{
			_permission_api := _api.Group("/permission_api", _permission_apiMw()...)
			{
				_coze_web_app := _permission_api.Group("/coze_web_app", _coze_web_appMw()...)
				_coze_web_app.POST("/impersonate_coze_user", append(_impersonatecozeuserMw(), coze.ImpersonateCozeUser)...)
			}
		}
		{
			_intelligence_api := _api.Group("/intelligence_api", _intelligence_apiMw()...)

			{
				_search := _intelligence_api.Group("/search", _searchMw()...)
				_search.POST("/get_draft_intelligence_info", append(_getdraftintelligenceinfoMw(), coze.GetDraftIntelligenceInfoByWanwu)...)
				_search.POST("/get_draft_intelligence_list", append(_getdraftintelligencelistMw(), coze.GetDraftIntelligenceListByWanwu)...)
			}
		}
		{
			_memory := _api.Group("/memory", _memoryMw()...)
			{
				_variable0 := _memory.Group("/variable", _variable0Mw()...)
				_variable0.POST("/get_meta", append(_getmemoryvariablemetaMw(), coze.GetMemoryVariableMetaByWanwu)...)
			}
			{
				_project := _memory.Group("/project", _projectMw()...)
				{
					_variable := _project.Group("/variable", _variableMw()...)
					_variable.GET("/meta_list", append(_getprojectvariablelistMw(), coze.GetProjectVariableListByWanwu)...)
				}
			}
			{
				_database := _memory.Group("/database", _databaseMw()...)
				_database.POST("/add", append(_adddatabaseMw(), coze.AddDatabase)...)
				_database.POST("/list", append(_listdatabaseMw(), coze.ListDatabaseByWanwu)...)
				_database.POST("/get_by_id", append(_getdatabasebyidMw(), coze.GetDatabaseByIDByWanwu)...)
				_database.POST("/list_records", append(_listdatabaserecordsMw(), coze.ListDatabaseRecords)...)
				_database.POST("/get_connector_name", append(_getconnectornameMw(), coze.GetConnectorNameByWanwu)...)
				_database.POST("/update_records", append(_updatedatabaserecordsMw(), coze.UpdateDatabaseRecords)...)
				_database.POST("/get_template", append(_getdatabasetemplateMw(), coze.GetDatabaseTemplate)...)
				_database.POST("/update", append(_updatedatabaseMw(), coze.UpdateDatabase)...)
				{
					_table := _database.Group("/table", _tableMw()...)
					_table.POST("/reset", append(_resetbottableMw(), coze.ResetBotTable)...)
				}
			}
		}
		{
			_knowledge0 := _api.Group("/knowledge", _knowledge0Mw()...)
			{
				_icon := _knowledge0.Group("/icon", _iconMw()...)
				_icon.POST("/get", append(_geticonfordatasetMw(), coze.GetIconForDatasetByWanwu)...)
			}
		}
		{
			_common := _api.Group("/common", _commonMw()...)
			{
				_upload := _common.Group("/upload", _uploadMw()...)
				_upload.GET("/apply_upload_action", append(_applyuploadactionMw(), coze.ApplyUploadActionByWanwu)...)
				_upload.POST("/apply_upload_action", append(_applyuploadaction0Mw(), coze.ApplyUploadActionByWanwu)...)
				_upload.POST("/*tos_uri", append(_commonuploadMw(), coze.CommonUpload)...)
			}
		}
		{
			_playground_api := _api.Group("/playground_api", _playground_apiMw()...)
			{
				_space := _playground_api.Group("/space", _spaceMw()...)
				_space.POST("/list", append(_getspacelistv2Mw(), wanwu_mock.GetSpaceListV2)...)
			}
		}
		{
			_plugin_api := _api.Group("/plugin_api", _plugin_apiMw()...)
			_plugin_api.POST("/get_playground_plugin_list", append(_getplaygroundpluginlistMw(), coze.GetPlaygroundPluginList)...)
		}
		{
			_developer := _api.Group("/developer", _developerMw()...)
			_developer.POST("/get_icon", append(_geticonMw(), coze.GetIconByWanwu)...)
		}
		{
			_workflow_api := _api.Group("/workflow_api", _workflow_apiMw()...)
			_workflow_api.GET("/apiDetail", append(_getapidetailMw(), coze.GetApiDetail)...)
			_workflow_api.POST("/batch_delete", append(_batchdeleteworkflowMw(), coze.BatchDeleteWorkflow)...)
			_workflow_api.POST("/cancel", append(_cancelworkflowMw(), coze.CancelWorkFlow)...)
			_workflow_api.POST("/canvas", append(_getcanvasinfoMw(), coze.GetLatestVersionCanvasInfoByWanwu)...)
			_workflow_api.POST("/canvas/draft", append(_getcanvasinfoMw(), coze.GetDraftCanvasInfoByWanwu)...)
			_workflow_api.POST("/copy", append(_copyworkflowMw(), coze.CopyWorkflowByWanwu)...)
			_workflow_api.POST("/copy_wk_template", append(_copywktemplateapiMw(), coze.CopyWkTemplateApi)...)
			_workflow_api.POST("/create", append(_createworkflowMw(), coze.CreateWorkflowByWanwu)...)
			_workflow_api.POST("/delete", append(_deleteworkflowMw(), coze.DeleteWorkflow)...)
			_workflow_api.POST("/delete_strategy", append(_getdeletestrategyMw(), coze.GetDeleteStrategy)...)
			_workflow_api.POST("/example_workflow_list", append(_getexampleworkflowlistMw(), coze.GetExampleWorkFlowListByWanwu)...)
			_workflow_api.GET("/get_node_execute_history", append(_getnodeexecutehistoryMw(), coze.GetNodeExecuteHistory)...)
			_workflow_api.GET("/get_process", append(_getworkflowprocessMw(), coze.GetWorkFlowProcess)...)
			_workflow_api.POST("/get_trace", append(_gettracesdkMw(), coze.GetTraceSDK)...)
			_workflow_api.POST("/history_schema", append(_gethistoryschemaMw(), coze.GetHistorySchemaByWanwu)...)
			_workflow_api.POST("/list_publish_workflow", append(_listpublishworkflowMw(), coze.ListPublishWorkflow)...)
			_workflow_api.POST("/list_spans", append(_listrootspansMw(), coze.ListRootSpans)...)
			_workflow_api.POST("/llm_fc_setting_detail", append(_getllmnodefcsettingdetailMw(), coze.GetLLMNodeFCSettingDetail)...)
			_workflow_api.POST("/llm_fc_setting_merged", append(_getllmnodefcsettingsmergedMw(), coze.GetLLMNodeFCSettingsMerged)...)
			_workflow_api.POST("/nodeDebug", append(_workflownodedebugv2Mw(), coze.WorkflowNodeDebugV2)...)
			_workflow_api.POST("/node_panel_search", append(_nodepanelsearchMw(), coze.NodePanelSearch)...)
			_workflow_api.POST("/node_template_list", append(_nodetemplatelistMw(), wanwu_mock.NodeTemplateListByWanwu)...)
			_workflow_api.POST("/node_type", append(_queryworkflownodetypesMw(), coze.QueryWorkflowNodeTypes)...)
			_workflow_api.POST("/publish", append(_publishworkflowMw(), coze.PublishWorkflow)...)
			_workflow_api.POST("/released_workflows", append(_getreleasedworkflowsMw(), coze.GetReleasedWorkflows)...)
			_workflow_api.POST("/save", append(_saveworkflowMw(), coze.SaveWorkflow)...)
			_workflow_api.POST("/sign_image_url", append(_signimageurlMw(), coze.SignImageURL)...)
			_workflow_api.POST("/test_resume", append(_workflowtestresumeMw(), coze.WorkFlowTestResume)...)
			_workflow_api.POST("/test_run", append(_workflowtestrunMw(), coze.WorkFlowTestRun)...)
			_workflow_api.POST("/update_meta", append(_updateworkflowmetaMw(), coze.UpdateWorkflowMeta)...)
			_workflow_api.POST("/convert_by_wanwu", append(_convertWorkflowMw(), coze.ConvertWorkflowByWanwu)...)
			_workflow_api.POST("/validate_tree", append(_validatetreeMw(), coze.ValidateTree)...)
			_workflow_api.POST("/workflow_detail", append(_getworkflowdetailMw(), coze.GetWorkflowDetail)...)
			_workflow_api.POST("/workflow_detail_info", append(_getworkflowdetailinfoMw(), coze.GetWorkflowDetailInfoByWanwu)...)
			_workflow_api.POST("/workflow_references", append(_getworkflowreferencesMw(), coze.GetWorkflowReferences)...)
			_workflow_api.POST("/version_list", append(_getworkflowversionlistMw(), coze.GetWorkflowVersionListByWanwu)...)
			_workflow_api.POST("/latest_version_list", append(_getworkflowlatestversionMw(), coze.MGetWorkflowLatestVersionByWanwu)...)
			_workflow_api.PUT("/version_description", append(_updateworkflowversiondescriptionMw(), coze.UpdateWorkflowVersionDescriptionByWanwu)...)
			_workflow_api.POST("/revert", append(_rollbackworkflowversionMw(), coze.RollbackWorkflowVersionByWanwu)...)
			_workflow_api.POST("/update_meta_by_wanwu", append(_updateworkflowmetaMw(), coze.UpdateWorkflowMetaByWanwu)...)

			{
				_chat_flow_role := _workflow_api.Group("/chat_flow_role", _chat_flow_roleMw()...)
				_chat_flow_role.POST("/create", append(_createchatflowroleMw(), coze.CreateChatFlowRole)...)
				_chat_flow_role.POST("/delete", append(_deletechatflowroleMw(), coze.DeleteChatFlowRole)...)
				_chat_flow_role.GET("/get", append(_getchatflowroleMw(), coze.GetChatFlowRole)...)
			}
			{
				_project_conversation := _workflow_api.Group("/project_conversation", _project_conversationMw()...)
				_project_conversation.GET("/list", append(_listprojectconversationdefMw(), coze.ListProjectConversationDef)...)
				_project_conversation.POST("/create_by_wanwu", append(_createprojectconversationdefMw(), coze.CreateProjectConversationDefByWanwu)...)
				// 先不动coze原有的DeleteProjectConversationDef接口(区别为删除的是dynamic conversation online)
				_project_conversation.POST("/delete", append(_deleteprojectconversationdefMw(), coze.DeleteProjectConversationDef)...)
				_project_conversation.POST("/delete_by_wanwu", append(_deleteprojectconversationdefMw(), coze.DeleteProjectConversationDefByWanwu)...)
			}
			{
				_upload1 := _workflow_api.Group("/upload", _upload1Mw()...)
				_upload1.POST("/auth_token", append(_getworkflowuploadauthtokenMw(), coze.GetWorkflowUploadAuthToken)...)
			}

			// --- wanwu adapt ---
			_workflow_api.POST("/workflow_list_by_wanwu", []app.HandlerFunc{coze.GetWorkFlowListByWanwu}...)
			_workflow_api.POST("/workflow_select_by_wanwu", []app.HandlerFunc{coze.GetWorkFlowSelectByWanwu}...)
			_workflow_api.POST("/import", []app.HandlerFunc{coze.ImportWorkFlow}...)
			_workflow_api.POST("/export", []app.HandlerFunc{coze.ExportWorkFlow}...)
		}
	}
	{
		_v1 := root.Group("/v1", _v1Mw()...)
		{
			_conversation0 := _v1.Group("/conversation", _conversation0Mw()...)
			{
				_message := _conversation0.Group("/message", _messageMw()...)
				_message.POST("/list", append(_getapimessagelistMw(), coze.GetApiMessageList)...)
				_message.POST("/list_by_wanwu", append(_getapimessagelistMw(), coze.GetApiMessageList)...) // 跳过coze的openapi鉴权
			}
		}
		{
			_workflows := _v1.Group("/workflows", _workflowsMw()...)
			_workflows.POST("/chat", append(_openapichatflowrunMw(), coze.OpenAPIChatFlowRunByWanwu)...)
			_workflows.GET("/:workflow_id", append(_openapigetworkflowinfoMw(), coze.OpenAPIGetWorkflowInfoByWanwu)...)
		}
		{
			_files := _v1.Group("/files", _filesMw()...)
			_files.POST("/upload", append(_uploadfileopenMw(), coze.UploadFileOpen)...)
		}
		{
			_workflow := _v1.Group("/workflow", _workflowMw()...)
			{
				_conversation1 := _workflow.Group("/conversation", _conversation1Mw()...)
				_conversation1.POST("/create", append(_openapicreateconversationMw(), coze.OpenAPICreateConversationByWanwu)...)
				_conversation1.POST("/create_by_wanwu", append(_openapicreateconversationMw(), coze.OpenAPICreateConversationByWanwu)...) // 跳过coze的openapi鉴权

			}
			// --- wanwu adapt ---
			_workflow.POST("/list_schema_by_wanwu", []app.HandlerFunc{coze.ListWorkFlowOpenAPIV3SchemaByWanwu}...)
			_workflow.GET("/:workflow_id/schema_by_wanwu", []app.HandlerFunc{coze.GetWorkFlowOpenAPIV3SchemaByWanwu}...)
			_workflow.POST("/:workflow_id/run_by_wanwu", []app.HandlerFunc{coze.OpenAPIRunWorkFlowByWanwu}...)
			_workflow.POST("/chat_by_wanwu", []app.HandlerFunc{coze.OpenAPIChatFlowRunByWanwu}...)
			_workflow.POST("/run_by_wanwu", []app.HandlerFunc{coze.RunWorkFlowLatestVersionByWanwu}...)
		}
	}
}
