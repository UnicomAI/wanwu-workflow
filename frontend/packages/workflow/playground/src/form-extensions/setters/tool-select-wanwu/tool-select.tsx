/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useEffect, useState, useRef } from 'react'
import { I18n } from '@coze-arch/i18n';
import { Modal } from '@coze-arch/coze-design';
import { IconCozEdit } from '@coze-arch/coze-design/icons';
import { SelectToolModal } from './components';
import { ToolSelectPluginSetting } from './components/tool-select-modal-wanwu/tool-select-plugin-setting';
import { ToolDatabaseSetting } from './components/tool-select-database-wanwu/tool-database-setting';
import { useWorkflowNode } from '@coze-workflow/base';
import { useDataSetInfos } from '@/hooks';

import { useGlobalState } from '@/hooks';
import { LibrarySelect } from '@/form-extensions/components/library-select-wanwu';
import { MetadataFilterModal, DEFAULT_METADATA } from '@/form-extensions/components/dataset-select-wanwu/metadata-filter-modal-wanwu';
import { getDefaultDatasetSetting } from '@/form-extensions/components/dataset-setting/utils';
import { TooltipAction } from '@/form-extensions/components/icon-name-desc-card-wanwu/tooltip-action';
import { IconNameDescCard } from '@/form-extensions/components/icon-name-desc-card-wanwu';

import { useModal } from './use-modal';
import { type ToolSelectValue, TOOL_TAB } from './types';

interface ToolListProps {
  value?: ToolSelectValue;
  onChange?: (newValue: ToolSelectValue) => void;
  readonly?: boolean;
  addButtonTestID?: string;
  libraryCardTestID?: string;
  form?: any;
  showDataset?: boolean | undefined;
  onlyShowSkill?: boolean | undefined;
}

export const ToolSelect = ({
  value,
  onChange,
  readonly = false,
  addButtonTestID,
  libraryCardTestID,
  form,
  showDataset,
  onlyShowSkill,
}: ToolListProps) => {
  const { spaceId, projectId, getProjectApi, playgroundProps } =
    useGlobalState();
  const {
    isVisible: isSelectDatabaseModalVisible,
    openModal: openSelectDatabaseModal,
    closeModal: closeSelectToolModal,
  } = useModal();
  const { data } = useWorkflowNode();
  const toolInfoList = data?.inputs?.toolInfoList;
  const [libraries, setLibraries] = useState<any>([]);
  const [metaVisible, setMetaVisible] = useState(false);
  const [curDatasetId, setCurDatasetId] = useState<string>();
  const [currentMetaData, setCurrentMetaData] = useState<any>(DEFAULT_METADATA);
  const [toolSettingVisible, setToolSettingVisible] = useState(false);
  const [currentTool, setCurrentTool] = useState<any>(null);
  const [tempApiKey, setTempApiKey] = useState<string>('');
  const latestApiKeyRef = useRef<string>('');
  const [databaseSettingVisible, setDatabaseSettingVisible] = useState(false);
  const [databaseSettingValue, setDatabaseSettingValue] = useState<any>(null);
  const [knowledgeId, setKnowledgeId] = useState<string>('');

  const prevValueRef = useRef<any>(null);
  const prevToolInfoListRef = useRef<any>(null);

  function handleLibrarySelectDelete(id: string) {
    Modal.confirm({
      title: I18n.t(
        'workflow_mcp_delete_confirm_modal_title' as any,
        {},
        '确认移除该条数据？',
      ),
      content: I18n.t(
        'workflow_mcp_delete_confirm_modal_content' as any,
        {},
        '移除后，该节点配置的相关内容均会被删除且无法恢复',
      ),
      onOk: () => {
        const currentValue = value || [];
        const nextValue = currentValue.filter((item: any) => {
          const itemId = getLibraryId(item, item.kind);
          return itemId !== id;
        });
        onChange?.(nextValue);
        setLibraries((libraries || []).filter((lib: any) => lib.id !== id));

        // Clear dataset setting when no knowledge base remains, allowing defaults to be re-applied on next add
        if (form && !nextValue.some((item: any) => item.kind === TOOL_TAB.DATABASE)) {
          form.setFieldValue('inputs.datasetSetting', {});
        }
      },
      okText: I18n.t('workflow_confirm_modal_ok', {}, '确定'),
      cancelText: I18n.t('workflow_confirm_modal_cancel', {}, '取消'),
    });
  }

  function handleLibraryToolDelete(id: string) {
     const currentValue = value || [];
     const nextValue = currentValue.filter((item: any) => {
        const itemId = getLibraryId(item, item.kind);
        return itemId !== id;
      });
      onChange?.(nextValue);
      setLibraries((libraries || []).filter((lib: any) => lib.id !== id));

      // Clear dataset setting when no knowledge base remains, allowing defaults to be re-applied on next add
      if (form && !nextValue.some((item: any) => item.kind === TOOL_TAB.DATABASE)) {
        form.setFieldValue('inputs.datasetSetting', {});
      }
  }

  function getLibraryId(itemData: any, kind: string): string {
    switch (kind) {
      case TOOL_TAB.TOOL:
        return itemData.api_id || '';
      case TOOL_TAB.WORKFLOW:
        return itemData.workflow_id || '';
      case TOOL_TAB.DATABASE:
        return itemData.knowledgeId || '';
      case TOOL_TAB.MCP:
        return itemData.name || '';
      case TOOL_TAB.SKILL:
        return itemData.skillId || '';
      default:
        return itemData.id || '';
    }
  }

  function handleSelectToolModalAdd(item: any) {
    const currentValue = value || [];
    const libraryId = getLibraryId(item.data, item.kind);
    switch (item?.kind) {
      case TOOL_TAB.TOOL:
        onChange?.([...currentValue, { ...item.data, kind: TOOL_TAB.TOOL, id: libraryId, description: item.data.desc, type: item.data.plugin_type}]);
        setLibraries([...(libraries || []), { ...item.data, kind: TOOL_TAB.TOOL, id: libraryId, description: item.data.desc, type: item.data.plugin_type }]);
        break;
      case TOOL_TAB.WORKFLOW:
        onChange?.([...currentValue, { ...item.data, kind: TOOL_TAB.WORKFLOW, id: libraryId, description: item.data.desc }]);
        setLibraries([...(libraries || []), { ...item.data, kind: TOOL_TAB.WORKFLOW, id: libraryId, description: item.data.desc }]);
        break;
      case TOOL_TAB.DATABASE: {
        const newDatabaseItem = { ...item.data, kind: TOOL_TAB.DATABASE, id: libraryId, description: item.data.desc };
        const nextDatabaseValue = [...currentValue, newDatabaseItem];
        onChange?.(nextDatabaseValue);
        setLibraries([...(libraries || []), newDatabaseItem]);

        // Initialize dataset setting with default values as soon as the first knowledge base is added,
        // so that the node already carries the full config before the user opens the setting modal.
        if (form) {
          const currentDatasetSetting = form.getValueIn('inputs.datasetSetting') || {};
          const hasValidSetting = Object.values(currentDatasetSetting).some(
            value => value !== undefined && value !== null,
          );
          if (!hasValidSetting) {
            form.setFieldValue('inputs.datasetSetting', getDefaultDatasetSetting([newDatabaseItem]));
          }
        }
        break;
      }
      case TOOL_TAB.MCP:
        onChange?.([...currentValue, { ...item.data, kind: TOOL_TAB.MCP, id: item.data.mcpId , description: item.data.description}]);
        setLibraries([...(libraries || []), { ...item.data, kind: TOOL_TAB.MCP, id: item.data.mcpId , description: item.data.description }]);
        break;
      case TOOL_TAB.SKILL:
        onChange?.([...currentValue, { ...item.data, kind: TOOL_TAB.SKILL, id: item.data.skillId, description: item.data.desc, name: item.data.skillName }]);
        setLibraries([...(libraries || []), { ...item.data, kind: TOOL_TAB.SKILL, id: item.data.skillId, description: item.data.desc, name: item.data.skillName }]);
        break;
      default:
        break;
    }
  }

  function handleEditLibrary(library: any) {
    const kind = library?.kind;
    if (kind === TOOL_TAB.DATABASE) {
      showEditMetaData(library);
      return;
    }
    if (kind === TOOL_TAB.TOOL) {
      const apiKey = library?.api_key || library?.apiKey || '';
      setCurrentTool(library);
      setTempApiKey(apiKey);
      latestApiKeyRef.current = apiKey;
      setToolSettingVisible(true);
      return;
    }
  }

  function canEdit(library: any) {
    const kind = library?.kind;
    const isBuiltin = library?.type === 'builtin';
    return (kind === TOOL_TAB.DATABASE && !library?.external) || (kind === TOOL_TAB.TOOL && isBuiltin);
  }

  const handleCloseMetaModal = () => {
    setMetaVisible(false);
  };

  const updateMetaDataForLibrary = (id: string, metaDataFilterParams: any) => {
    setLibraries((prev: any[]) =>
      (prev || []).map(lib =>
        lib.id === id ? { ...lib, metaDataFilterParams } : lib,
      ),
    );
    const currentValue = value || [];
    const updatedValue = currentValue.map((item: any) => {
      const itemId = getLibraryId(item, item.kind);
      if (itemId === id) {
        return { ...item, metaDataFilterParams };
      }
      return item;
    });
    onChange?.(updatedValue);
  };

  const updateApiKeyForTool = (id: string, apiKey: string) => {
    setLibraries((prev: any[]) =>
      (prev || []).map(lib =>
        lib.id === id ? { ...lib, api_key: apiKey, apiKey } : lib,
      ),
    );
    const currentValue = value || [];
    const updatedValue = currentValue.map((item: any) => {
      const itemId = getLibraryId(item, item.kind);
      if (itemId === id) {
        return { ...item, api_key: apiKey, apiKey };
      }
      return item;
    });
    onChange?.(updatedValue);
  };

  const updateDatabaseSetting = (databaseSetting: any) => {
    setDatabaseSettingValue(databaseSetting);
    if (form) {
      form.setFieldValue('inputs.datasetSetting', databaseSetting);
    }
    setDatabaseSettingVisible(false);
  };

  const showEditMetaData = async (library: any) => {
    const { id, knowledgeId, metaDataFilterParams } = library || {};
    if (!knowledgeId && !id) return;

    setCurDatasetId(id);
    setKnowledgeId(knowledgeId || id);
    setCurrentMetaData(metaDataFilterParams || DEFAULT_METADATA);
    setMetaVisible(true);
  };


  useEffect(() => {
    const valueChanged = JSON.stringify(value) !== JSON.stringify(prevValueRef.current);
    const toolInfoListChanged = JSON.stringify(toolInfoList) !== JSON.stringify(prevToolInfoListRef.current);
    if (value && valueChanged) {
    const formattedLibraries = (value || []).map(item => ({
      ...item,
      id: getLibraryId(item, (item as any).kind)
    }));
    setLibraries(formattedLibraries || []);
    prevValueRef.current = value;
  } else if (toolInfoList && toolInfoListChanged) {
    const formattedLibraries = (toolInfoList || []).map(item => ({
      ...item,
      id: getLibraryId(item, item.kind)
    }));
    setLibraries(formattedLibraries || []);
    prevToolInfoListRef.current = toolInfoList;
  }
  }, [value, toolInfoList]);


  const isHideSettingButton = !showDataset || !libraries.some((lib: any) => lib?.kind === 'database');
  const dataSetLibraries = libraries.filter((item: any) => item?.kind === 'database');
  const { dataSets } = useDataSetInfos({ ids: dataSetLibraries.map(item => item.dataset_id) });

  const newLibraries = showDataset
    ? dataSets.map((item: any, index: number) => ({...item, ...dataSetLibraries[index]}))
    : libraries.filter((item: any) => item?.kind !== 'database');

  return (
    <>
      <LibrarySelect
        readonly={readonly}
        libraries={newLibraries}
        onAddLibrary={openSelectDatabaseModal}
        onSettingLibrary={() => {
          let datasettingValue = databaseSettingValue;
          if ((!datasettingValue || Object.keys(datasettingValue).length === 0) && form) {
            datasettingValue = form.getValueIn('inputs.datasetSetting') || {};
          }
          setDatabaseSettingValue(datasettingValue);
          setDatabaseSettingVisible(true);
        }}
        settingLibraryTooltip={I18n.t('chatflow_agent_skill_knowledge_setting_tooltip' as any, {}, '知识库设置')}
        hideSettingButton={isHideSettingButton}
        onDeleteLibrary={handleLibrarySelectDelete}
        onEditLibrary={(id: string) => {
          const lib = (newLibraries || []).find((item: any) => item?.id === id);
          if (!canEdit(lib)) return;
          handleEditLibrary(lib);
        }}
        renderLibrary={({ library }) => {
          const libAny = library as any;
          const editTooltip =
            libAny?.kind === TOOL_TAB.DATABASE
              ? I18n.t('datasets_metadata_filter')
              : I18n.t('edit_params' as any, {}, '编辑参数');
          const { orgName, share, category, external } = libAny || {};
          const extraInfo = { orgName, share, category, external };
          const canEditLib = canEdit(libAny);
          const actions = canEditLib ? (
            <TooltipAction
              tooltip={editTooltip}
              icon={<IconCozEdit />}
              onClick={() => handleEditLibrary(libAny)}
              testID={`${libraryCardTestID || ''}.editData`}
            />
          ) : null;

          return (
            <IconNameDescCard
              name={libAny?.name}
              nameSuffix={libAny?.nameExtra}
              description={libAny?.description}
              extraInfo={extraInfo}
              isTagHidden={!showDataset}
              icon={null}
              onRemove={() => handleLibrarySelectDelete(getLibraryId(libAny, libAny.kind))}
              onEdit={canEditLib ? () => handleEditLibrary(libAny) : undefined}
              readonly={readonly}
              actions={actions}
              showEditBtn={false}
              showDeleteBtn
              testID={libraryCardTestID}
            />
          )
        }}
        emptyText={
          showDataset
            ? I18n.t('workflow_knowledge_node_empty')
            : onlyShowSkill
              ? I18n.t('workflow_skill_node_skills_empty')
              : I18n.t('workflow_agnet_tool_database_empty')
        }
        addButtonTestID={addButtonTestID}
        libraryCardTestID={libraryCardTestID}
      />
      <MetadataFilterModal
        visible={metaVisible}
        handleClose={handleCloseMetaModal}
        defaultValue={currentMetaData}
        knowledgeId={knowledgeId}
        onSubmit={(metaDataFilterParams) => {
          if (curDatasetId) {
            updateMetaDataForLibrary(curDatasetId, metaDataFilterParams);
          }
          handleCloseMetaModal();
        }}
      />

      <SelectToolModal
        spaceId={spaceId}
        visible={isSelectDatabaseModalVisible}
        onClose={closeSelectToolModal}
        onAddTool={handleSelectToolModalAdd}
        onRemoveTool={handleLibraryToolDelete}
        enterFrom={TOOL_TAB.WORKFLOW}
        projectID={projectId}
        showDataset={showDataset}
        onlyShowSkill={onlyShowSkill}
      />

      <ToolSelectPluginSetting
        visible={toolSettingVisible}
        value={tempApiKey}
        onChange={(apiKey) => {
          setTempApiKey(apiKey);
          latestApiKeyRef.current = apiKey;
        }}
        onOk={() => {
          if (currentTool?.id) {
            const toolId = getLibraryId(currentTool, currentTool.kind);
            updateApiKeyForTool(toolId, latestApiKeyRef.current);
          }
          setToolSettingVisible(false);
          setCurrentTool(null);
          setTempApiKey('');
          latestApiKeyRef.current = '';
        }}
        onCancel={() => {
          setToolSettingVisible(false);
          setCurrentTool(null);
          setTempApiKey('');
          latestApiKeyRef.current = '';
        }}
        readonly={readonly}
      />

      <ToolDatabaseSetting
        visible={databaseSettingVisible}
        value={databaseSettingValue || {}}
        showUseGraph={newLibraries.some((lib: any) => lib?.kind === TOOL_TAB.DATABASE && lib?.graphSwitch === true)}
        selectDataSet={newLibraries.filter((lib: any) => lib?.kind === TOOL_TAB.DATABASE)}
        onChange={(v) => {
          setDatabaseSettingValue({ ...databaseSettingValue, ...v });
        }}
        onOk={() => {
          updateDatabaseSetting(databaseSettingValue);
        }}
        onCancel={() => {
          setDatabaseSettingVisible(false);
        }}
        readonly={readonly}
      />
    </>
  );
};