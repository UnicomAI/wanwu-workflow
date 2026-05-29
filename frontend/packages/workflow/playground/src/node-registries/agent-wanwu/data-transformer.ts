import { get, camelCase } from 'lodash-es';
import { type NodeFormContext } from '@flowgram-adapter/free-layout-editor';
import { type NodeDataDTO, type InputValueDTO, BlockInput, type InputValueVO, type DTODefine, ValueExpressionType } from '@coze-workflow/base';
import { variableUtils } from '@coze-workflow/variable';
import { ModelParamType, type Model } from '@coze-arch/bot-api/developer_api';

import { type FormData } from './types';
import { OUTPUTS } from './constants';
import { WorkflowModelsService } from '@/services';
import { getDefaultLLMParams, reviseLLMParamPair } from '@/nodes-v2/llm-wanwu/utils';
import { type IModelValue } from '@/typing';
import { normalizeInputParameters, parseLLMParam, transformRefContent } from './util';

const formatKnowledgeList = (knowledgeList) => {
  return knowledgeList.map(item => {
    if (typeof item === 'string') {
      return {
        name: item,
        dataset_id: item
      }
    } else {
      return item
    }
  })
}

/**
 * 节点后端数据 -> 前端表单数据
 */
export function transformOnInit (value: FormData,context: NodeFormContext) {
  const { node, playgroundContext } = context;
  const { variableService } = playgroundContext; 
  //模型参数
  const modelsService = node.getService<WorkflowModelsService>(
    WorkflowModelsService,
  );
  const models = modelsService.getModels();
  let llmParam = get(value, 'inputs.llmParam') as InputValueDTO[] | undefined;
  if (!llmParam) {
    llmParam = getDefaultLLMParams(models);
  }
  const normalizedLLMParam = normalizeInputParameters(llmParam);
  const modelParams = parseLLMParam(normalizedLLMParam);
  const { systemPrompt, ...model } = modelParams;
  const modelValue = model.modelType ? { ...model } : undefined;

  //输入参数
  const inputParameters = get(value, 'inputs.inputParameters', []) as InputValueDTO[]; 
  //数据库参数
  const datasetParam = get(value, 'inputs.datasetParam', []) as InputValueDTO[];
  const datasetSetting = {
    topK: datasetParam.find(item => item.name === 'topK')?.input.value
      .content as number,

    threshold: datasetParam.find(item => item.name === 'threshold')?.input.value
      .content as number,

    semanticsPriority: datasetParam.find(item => item.name === 'semanticsPriority')?.input.value
      .content as number,

    maxHistory: datasetParam.find(item => item.name === 'maxHistory')?.input.value
      .content as number,

    rerankKeywordPriority: datasetParam.find(item => item.name === 'rerankKeywordPriority')?.input.value
      .content as number,

    rerankKeywordPrioritySwitch: datasetParam.find(item => item.name === 'rerankKeywordPrioritySwitch')?.input
      .value.content as boolean,

    useGraph: datasetParam.find(item => item.name === 'useGraph')?.input
      .value.content as boolean,

    matchType: datasetParam.find(item => item.name === 'matchType')?.input.value
      .content as string,

    rerankModelId: datasetParam.find(item => item.name === 'rerankModelId')?.input.value
      .content as string,

    rewrite: datasetParam.find(item => item.name === 'rewrite')?.input
      .value.content as boolean,
  };
  const newKnowledgeList = formatKnowledgeList(datasetParam[0]?.input?.value?.content || []);
  const datasetSelectParam = (() =>{
    const rawData = newKnowledgeList;
    if (!Array.isArray(rawData)) return [];
    return rawData.map(item => {
      if (typeof item !== 'object' || item === null) {
        return { kind: 'database', id: '', name: '', description: '' };
      }
      return {
        ...item,
        kind: 'database',
        id: String(item.knowledgeId || '')
      };
    });
  })() as any[];

  //workflow
  const agentWorkflowParams = (() => {
  const rawData = get(value, 'inputs.agentWorkflowParams', []);
  if (!Array.isArray(rawData)) return [];
  return rawData.map(item => {
    if (typeof item !== 'object' || item === null) {
      return { kind: 'workflow', id: '', name: '', description: '' };
    }
    return {
      ...(item as any),
      kind: 'workflow',
      id: String((item as any).workflowId || ''),
      name: String((item as any).workflowName || '')
    };
  });
})() as any[];
  //tool
  const agentToolParams = (() => {
    const rawData = get(value, 'inputs.agentToolParams', []);
    if (!Array.isArray(rawData)) return [];
    return rawData.map(item => {
      if (typeof item !== 'object' || item === null) {
        return { kind: 'tool', id: '', name: '', description: '', type: '' };
      }
      return {
        ...(item as any),
        kind: 'tool',
        id: String((item as any).actionID || ''),
        name: String((item as any).actionName || ''),
        type: String((item as any).toolType || ''),
      };
    });
  })() as any[];
  //mcp
  const agentMCPParams = (() => {
    const rawData = get(value, 'inputs.agentMCPParams', []);
    if (!Array.isArray(rawData)) return [];
    return rawData.map(item => {
      if (typeof item !== 'object' || item === null) {
        return { kind: 'mcp', id: '', name: '', description: '' };
      }
      return {
        ...(item as any),
        kind: 'mcp',
        id: String((item as any).mcpId || ''),
        name: String((item as any).mcpToolName || '')
      };
    });
  })() as any[];
  //skill
  const agentSkillParams = (() => {
    const rawData = get(value, 'inputs.agentSkillParams', []);
    if (!Array.isArray(rawData)) return [];
    return rawData.map(item => {
      if (typeof item !== 'object' || item === null) {
        return { kind: 'skill', id: '', name: '', description: '' };
      }
      return {
        ...(item as any),
        kind: 'skill',
        id: String((item as any).skillId || ''),
        name: String((item as any).skillName || ''),
        description: String((item as any).desc || '')
      };
    });
  })() as any[];

  const toolInfoList = [
    ...agentToolParams,
    ...agentWorkflowParams,
    ...agentMCPParams,
    ...datasetSelectParam,
    ...agentSkillParams
  ];
  const outputs = value?.outputs ?? OUTPUTS;
  return {
    nodeMeta: value?.nodeMeta,
    inputs: {
      inputParameters,
      systemPrompt,
      llmParam: modelValue,
      toolInfoList,
      datasetSetting
    },
    outputs,
  };
};

/**
 * 前端表单数据 -> 节点后端数据
 */
export function transformOnSubmit (value,context: NodeFormContext){
  const { node, playgroundContext } = context;
  const toolInfoList = get(value, 'inputs.toolInfoList', []) as any[];
  const agentToolParams: any[] = [];
  const agentWorkflowParams: any[] = [];
  const agentMCPParams: any[] = [];
  const agentSkillParams: any[] = [];

  toolInfoList.forEach(tool => {
    if (tool.kind === 'workflow') {
      agentWorkflowParams.push({
        ...tool,
        workflowId: tool.id || tool.workflow_id || tool.workflowId,
        workflowName: tool.name || tool.workflowName,
        description: tool.description || tool.desc,
      });
    } else if (tool.kind === 'mcp') {
      agentMCPParams.push({
        ...tool,
        mcpId: tool.id,
        mcpType: tool.mcpType,
        mcpToolName: tool.name,
        description: tool.description,
      });
    } else if (tool.kind === 'tool') {
      agentToolParams.push({
        ...tool,
        actionID: tool.api_id,
        actionName: tool.name,
        toolType: tool.plugin_type,
        apiKey: tool.apiKey,
        toolId: tool.plugin_id,
        toolName: tool.plugin_name,
        description: tool.desc,
      });
    } else if (tool.kind === 'skill') {
      agentSkillParams.push({
        ...tool,
        skillId: tool.id,
        skillName: tool.name,
        description: tool.description || tool.desc,
      });
    }
  });
  const databaseTools = toolInfoList.filter(tool => tool.kind === 'database');
  const datasetParam: any[] = databaseTools.length > 0 ? [{
    name: "knowledgeList",
    input: {
      type: "list",
      schema: {
        type: "string"
      },
      value: {
        type: "literal",
        content: databaseTools || []
      }
    }
  }] : [];

  const databaseSetting = get(value, 'inputs.datasetSetting', {});
  if (databaseTools.length > 0 && databaseSetting) {
    if (databaseSetting.topK !== undefined) {
      datasetParam.push({
        name: 'topK',
        input: {
          type: 'integer',
          value: {
            type: 'literal',
            content: databaseSetting.topK,
          },
        },
      });
    }
    if (databaseSetting.threshold !== undefined) {
      datasetParam.push({
        name: 'threshold',
        input: {
          type: 'float',
          value: {
            type: 'literal',
            content: databaseSetting.threshold,
          },
        },
      });
    }
    if (databaseSetting.semanticsPriority !== undefined) {
      datasetParam.push({
        name: 'semanticsPriority',
        input: {
          type: 'float',
          value: {
            type: 'literal',
            content: databaseSetting.semanticsPriority,
          },
        },
      });
    }
    if (databaseSetting.maxHistory !== undefined) {
      datasetParam.push({
        name: 'maxHistory',
        input: {
          type: 'integer',
          value: {
            type: 'literal',
            content: databaseSetting.maxHistory,
          },
        },
      });
    }
    if (databaseSetting.rerankKeywordPriority !== undefined) {
      datasetParam.push({
        name: 'rerankKeywordPriority',
        input: {
          type: 'float',
          value: {
            type: 'literal',
            content: databaseSetting.rerankKeywordPriority,
          },
        },
      });
    }
    if (databaseSetting.rerankKeywordPrioritySwitch !== undefined) {
      datasetParam.push(BlockInput.createBoolean('rerankKeywordPrioritySwitch', databaseSetting.rerankKeywordPrioritySwitch));
    }
    if (databaseSetting.useGraph !== undefined) {
      datasetParam.push(BlockInput.createBoolean('useGraph', databaseSetting.useGraph));
    }
    if (databaseSetting.matchType !== undefined && databaseSetting.matchType !== null) {
      datasetParam.push({
        name: 'matchType',
        input: {
          type: 'string',
          value: {
            type: 'literal',
            content: databaseSetting.matchType,
          },
        },
      });
    }
    if (databaseSetting.rerankModelId !== undefined && databaseSetting.rerankModelId !== null) {
      datasetParam.push({
        name: 'rerankModelId',
        input: {
          type: 'string',
          value: {
            type: 'literal',
            content: databaseSetting.rerankModelId,
          },
        },
      });
    }
    if (databaseSetting.rewrite !== undefined) {
      datasetParam.push(BlockInput.createBoolean('rewrite', databaseSetting.rewrite));
    }
  }


  const llmParam: any[] = [];
  const modelsService = node.getService<WorkflowModelsService>(
    WorkflowModelsService,
  );
  const models = modelsService?.getModels() ?? [];
  const model = get(value, 'inputs.llmParam') as IModelValue | undefined;
  const modelType = model?.modelType;
  const modelMeta = modelType
    ? models.find(m => m.model_type === modelType)
    : undefined;
  
  if (model?.modelType) {
    llmParam.push(
      BlockInput.createInteger('modelType', String(model.modelType)),
    );
  }

  if (model) {
    const excludeKeys = ['modelType', 'modelName', 'generationDiversity', 'responseFormat'];
    Object.keys(model).forEach(k => {
      if (excludeKeys.includes(k)) {
        return;
      }
      const paramValue = model[k];
      if (paramValue === undefined || paramValue === null) {
        return;
      }
      
      const paramDef = modelMeta?.model_params?.find(
        p => camelCase(p.name) === k,
      );
      const paramType = paramDef?.type;
      
      if (ModelParamType.Float === paramType) {
        llmParam.push(BlockInput.createFloat(k, String(paramValue)));
      } else if (ModelParamType.Int === paramType || ['modelType'].includes(k)) {
        llmParam.push(BlockInput.createInteger(k, String(paramValue)));
      } else {
        let _k = k;
        if (_k === 'modelName') {
          _k = 'modleName';
        }
        llmParam.push(BlockInput.createString(_k, String(paramValue)));
      }
    });
  }

  const systemPrompt = get(value, 'inputs.systemPrompt') as string | undefined;
  if (systemPrompt) {
    llmParam.push(
      BlockInput.createString('systemPrompt', String(systemPrompt)),
    );
  }

  // 转换输入参数
  const inputParameters = get(value, 'inputs.inputParameters', []);
  const safeInputParameters = Array.isArray(inputParameters) ? inputParameters : [];
 
  return {
    nodeMeta: value?.nodeMeta,
    inputs: {
      inputParameters: safeInputParameters,
      llmParam,
      datasetParam,
      agentToolParams,
      agentWorkflowParams,
      agentMCPParams,
      agentSkillParams,
    },
    outputs: value.outputs,
  } as NodeDataDTO;
};
