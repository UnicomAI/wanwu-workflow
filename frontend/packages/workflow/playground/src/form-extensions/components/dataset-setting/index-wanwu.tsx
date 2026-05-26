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

/* eslint-disable complexity */
import { type FC, useEffect, useState, useMemo } from 'react';

import { isNil, set } from 'lodash-es';
import { useNodeTestId } from '@coze-workflow/base';
import { type Dataset, FormatType } from '@coze-arch/idl/knowledge';
import { I18n } from '@coze-arch/i18n';
import { IconWarningInfo } from '@coze-arch/bot-icons';
import { Popover } from '@coze-arch/coze-design';

import { CheckboxWithLabel } from '../checkbox-with-label';
import { MatchType, type DataSetInfo } from './type';
import { TitleArea, SliderArea, SearchStrategyWanwu, RerankModelWanwu } from './components';

import s from './index.module.less';

/** Prompt beyond this value */
const SUGGEST_TOP_K = 5;
/** default minimum match */
const DEFAULT_MIN_SCORE = 0.4;

const DEFAULT_SEMANTICS_PRIORITY = 0.2;
const DEFAULT_KEYWORD_PRIORITY = 1;
const DEFAULT_MAX_HISTORY = 0;
/** default maximum recall  */
const DEFAULT_TOP_K = 5;
const DEFAULT_MATCH_TYPE = MatchType.HybirdPriority;

export interface DataSetSettingProps {
  selectDataSet: any;
  dataSetInfo: DataSetInfo;
  onDataSetInfoChange: (v: DataSetInfo) => void;
  readonly?: boolean;
  disabled?: boolean;
  style?: Record<string, unknown>;
  isReady?: boolean;
  dataSets?: Dataset[];
}

export const DataSetSetting: FC<DataSetSettingProps> = ({
  selectDataSet,
  dataSetInfo,
  onDataSetInfoChange,
  readonly,
  disabled,
  style,
  isReady,
  dataSets,
}) => {
  const {
    threshold,
    maxHistory,
    rerankKeywordPriority,
    rerankKeywordPrioritySwitch,
    topK,
    matchType,
    rerankModelId,
    semanticsPriority,
    rewrite,
    useGraph,
  } = dataSetInfo || {};

  const isAllExternalKnowledge = selectDataSet.every(item => item?.external)
  const isShowGraph = selectDataSet.some(item => item.graphSwitch)

  const isDatasetWriteActive = true;
  const isDatasetKeywordPrioritySwitch = false;
  const isDatasetGraphActive = true;

  const [isDatasetEmpty, setDatasetEmpty] = useState(true);
  const [isInit, setIsInit] = useState(true);

  const isContainSqlDataSet = useMemo(
    () => dataSets?.some(dataset => dataset?.format_type === FormatType.Table),
    [dataSets],
  );

  const { getNodeSetterId } = useNodeTestId();

  // Set default value
  useEffect(() => {
    if (isDatasetEmpty) {
      return;
    }

    if (
      isNil(threshold) &&
      isNil(maxHistory) &&
      isNil(rerankKeywordPriority) &&
      isNil(rerankKeywordPrioritySwitch) &&
      isNil(useGraph) &&
      isNil(semanticsPriority) &&
      isNil(topK) &&
      isNil(matchType) &&
      isNil(rerankModelId) &&
      isNil(rewrite)
    ) {
      const initDataSetInfo = {
        threshold: DEFAULT_MIN_SCORE,
        topK: DEFAULT_TOP_K,
        matchType: DEFAULT_MATCH_TYPE,
        rerankModelId: '',
        maxHistory: DEFAULT_MAX_HISTORY,
        rerankKeywordPriority: DEFAULT_KEYWORD_PRIORITY,
        semanticsPriority: DEFAULT_SEMANTICS_PRIORITY
      };

      if (isDatasetWriteActive) {
        set(initDataSetInfo, 'rewrite', !isAllExternalKnowledge);
      }

      if (isDatasetGraphActive) {
        set(initDataSetInfo, 'useGraph', isShowGraph);
      }

      if (!isDatasetKeywordPrioritySwitch) {
        set(initDataSetInfo, 'rerankKeywordPrioritySwitch', false);
      }

      // The search policy for new nodes defaults to Hybird
      onDataSetInfoChange?.({
        ...dataSetInfo,
        ...initDataSetInfo,
      });
    } else if (isNil(matchType)) {
      // The search policy for existing processes defaults to Hybird
      onDataSetInfoChange?.({
        ...dataSetInfo,
        matchType: DEFAULT_MATCH_TYPE,
      });
    } else if (
      // stock process supplement default value
      isNil(rewrite) &&
      isDatasetWriteActive
    ) {
      onDataSetInfoChange?.({
        ...dataSetInfo,
        rewrite: true,
      });
    } else if (
      isNil(rerankKeywordPrioritySwitch) &&
      isDatasetKeywordPrioritySwitch
    ) {
      onDataSetInfoChange?.({
        ...dataSetInfo,
        rerankKeywordPrioritySwitch: true,
      });
    } else if (
      isNil(useGraph) &&
      isDatasetGraphActive
    ) {
      onDataSetInfoChange?.({
        ...dataSetInfo,
        useGraph: true,
      });
    }
  }, [
    dataSetInfo,
    isDatasetWriteActive,
    isDatasetKeywordPrioritySwitch,
    isDatasetGraphActive,
    isContainSqlDataSet,
    isDatasetEmpty
  ]);

  useEffect(() => {
    if (!isInit && !isDatasetEmpty) {
      onDataSetInfoChange?.({
        ...dataSetInfo,
        useGraph: isShowGraph,
        rewrite: !isAllExternalKnowledge,
      });
    }
    setIsInit(false)
  }, [isShowGraph, isAllExternalKnowledge])

  useEffect(() => {
    if (!isReady) {
      return;
    }
    // Empty data when no database exists
    setDatasetEmpty(!dataSets?.length);
    if (!dataSets?.length) {
      const nextDataSetInfo = {};
      onDataSetInfoChange(nextDataSetInfo as DataSetInfo);
    }
  }, [dataSets, isReady, setDatasetEmpty]);

  const [topKSuggestVisible, setTopKSuggestVisible] = useState(false);

  const suggestTopKPopover = () => (
    <div className={s['tip-area']}>
      <IconWarningInfo></IconWarningInfo>
      <div className={s['tip-area-content']}>
        <div className={s['tip-area-title']}>
          {I18n.t('workflow_detail_knowledge_proceed_with_caution')}
        </div>
        <div className={s['tip-area-text']}>
          {I18n.t('dataset_max_recall_desc')}
        </div>
      </div>
    </div>
  );

  if (isDatasetEmpty) {
    return <></>;
  }

  return (
    // Set the positioning to prevent the slider from overshifting
    <div className={s.setting} style={{...style, position: 'relative'}}>
      {!isAllExternalKnowledge && (
        <div className={s['setting-item']}>
          <TitleArea
            title={I18n.t('knowledge_search_strategy_title')}
            tip={I18n.t('knowledge_search_strategy_tooltip')}
          />
          <SearchStrategyWanwu
            readonly={readonly}
            value={matchType as MatchType}
            onChange={v => {
              onDataSetInfoChange(
                {
                  ...dataSetInfo,
                  matchType: v,
                },
              );
            }}
          />
        </div>
      )}

      {matchType !== MatchType.HybirdPriority && !isAllExternalKnowledge && (
        <div className={s['setting-item']}>
          <TitleArea
            title={I18n.t('knowledge_rerank')}
          />
          <RerankModelWanwu
            readonly={readonly}
            value={rerankModelId as string}
            onChange={v => {
              onDataSetInfoChange(
                {
                  ...dataSetInfo,
                  rerankModelId: v,
                },
              );
            }}
          />
        </div>
      )}

      {matchType === MatchType.HybirdPriority && !isAllExternalKnowledge && (
        <div className={s['setting-item']}>
          <TitleArea
            title={
              `${I18n.t('dataset_lang')}${semanticsPriority} / ${I18n.t('dataset_keywords')}${1 - (semanticsPriority || 0)}`
            }
          />
          <div style={{position: 'relative'}}>
            <SliderArea
              min={0}
              max={1}
              step={0.01}
              customStyles={{
                sliderAreaStyle: {
                  width: '160px',
                },
                boundaryStyle: {
                  width: '158px',
                  margin: 0,
                },
              }}
              isDataSet
              value={semanticsPriority as number}
              marks={{markKey: DEFAULT_SEMANTICS_PRIORITY, markText: ''}}
              disabled={readonly || disabled}
              onChange={v => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  semanticsPriority: v,
                });
              }}
              onClickDefault={() => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  semanticsPriority: DEFAULT_SEMANTICS_PRIORITY,
                });
              }}
            />
          </div>
        </div>
      )}

      <div className={s['setting-item']}>
        <TitleArea
          title={'TopK'}
          tip={I18n.t('bot_edit_datasetsSettings_topK')}
        />
        <Popover
          showArrow={false}
          position="bottom"
          trigger="custom"
          visible={topKSuggestVisible}
          content={suggestTopKPopover}
          onClickOutSide={() => {
            setTopKSuggestVisible(false);
          }}
          className={s['dataset-top-k-popover']}
          getPopupContainer={() => document.body}
        >
          <div style={{position: 'relative'}}>
            <SliderArea
              min={0}
              max={20}
              step={1}
              value={topK || 0}
              customStyles={{
                sliderAreaStyle: {
                  width: '160px',
                },
                boundaryStyle: {
                  width: '158px',
                  margin: 0,
                },
              }}
              isDataSet
              marks={{
                markKey: DEFAULT_TOP_K,
                // Set margin-left to avoid overlap with number 1
                markText: '',
              }}
              onChange={v => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  topK: v,
                });
                if (v > SUGGEST_TOP_K) {
                  setTopKSuggestVisible(true);
                } else {
                  setTopKSuggestVisible(false);
                }
              }}
              onClickDefault={() => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  topK: DEFAULT_TOP_K,
                });
              }}
              disabled={readonly || disabled}
            />
          </div>
        </Popover>
      </div>

      {/*暂时不展示*/}
      {/*{matchType !== MatchType.HybirdPriority && (<div className={s['setting-item']}>
        <TitleArea
          title={I18n.t('dataset_content_length')}
          tip={I18n.t('bot_edit_datasetsSettings_content_length')}
        />
        <div style={{position: 'relative'}}>
          <SliderArea
            min={0}
            max={100}
            step={1}
            customStyles={{
              sliderAreaStyle: {
                width: '160px',
              },
              boundaryStyle: {
                width: '158px',
                margin: 0,
              },
            }}
            isDataSet
            value={maxHistory as number}
            marks={{markKey: DEFAULT_MAX_HISTORY, markText: ''}}
            disabled={readonly || disabled}
            onChange={v => {
              onDataSetInfoChange({
                ...dataSetInfo,
                maxHistory: v,
              });
            }}
            onClickDefault={() => {
              onDataSetInfoChange({
                ...dataSetInfo,
                maxHistory: DEFAULT_MAX_HISTORY,
              });
            }}
          />
        </div>
      </div>)}*/}

      <div className={s['setting-item']}>
        <TitleArea
          title={I18n.t('dataset_score')}
          tip={I18n.t('bot_edit_datasetsSettings_score')}
        />
        <div style={{position: 'relative'}}>
          <SliderArea
            min={0}
            max={1}
            step={0.01}
            customStyles={{
              sliderAreaStyle: {
                width: '160px',
              },
              boundaryStyle: {
                width: '158px',
                margin: 0,
              },
            }}
            isDataSet
            value={threshold as number}
            marks={{markKey: DEFAULT_MIN_SCORE, markText: ''}}
            disabled={readonly || disabled}
            onChange={v => {
              onDataSetInfoChange({
                ...dataSetInfo,
                threshold: v,
              });
            }}
            onClickDefault={() => {
              onDataSetInfoChange({
                ...dataSetInfo,
                threshold: DEFAULT_MIN_SCORE,
              });
            }}
          />
        </div>
      </div>

      {/*暂时不展示*/}
      {/*{matchType === MatchType.Hybird && (<>
        <div className={s['setting-item']}>
          <CheckboxWithLabel
            checked={rerankKeywordPrioritySwitch}
            onChange={checked => {
              onDataSetInfoChange({
                ...dataSetInfo,
                rerankKeywordPrioritySwitch: checked,
              });
            }}
            readonly={readonly}
            label={I18n.t('dataset_keyword_priority_switch')}
            tooltip={I18n.t('bot_edit_datasetsSettings_keyword_priority_switch')}
            dataTestId={getNodeSetterId('dataset_keyword_priority_switch')}
          />
        </div>
        {rerankKeywordPrioritySwitch && (<div className={s['setting-item']}>
          <TitleArea
            title={I18n.t('dataset_keyword_priority')}
            tip={I18n.t('bot_edit_datasetsSettings_keyword_priority')}
          />
          <div style={{position: 'relative'}}>
            <SliderArea
              min={0}
              max={2}
              step={0.1}
              customStyles={{
                sliderAreaStyle: {
                  width: '160px',
                },
                boundaryStyle: {
                  width: '158px',
                  margin: 0,
                },
              }}
              isDataSet
              value={rerankKeywordPriority as number}
              marks={{markKey: DEFAULT_KEYWORD_PRIORITY, markText: ''}}
              disabled={readonly || disabled}
              onChange={v => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  rerankKeywordPriority: v,
                });
              }}
              onClickDefault={() => {
                onDataSetInfoChange({
                  ...dataSetInfo,
                  rerankKeywordPriority: DEFAULT_KEYWORD_PRIORITY,
                });
              }}
            />
          </div>
        </div>)}
      </>)}*/}

      {!isAllExternalKnowledge &&(
        <div className={s['setting-item']}>
          <CheckboxWithLabel
            checked={rewrite}
            onChange={checked => {
              onDataSetInfoChange({
                ...dataSetInfo,
                rewrite: checked,
              });
            }}
            readonly={readonly}
            label={I18n.t('dataset_keywords_search')}
            description={I18n.t('bot_edit_datasetsSettings_keywords')}
            dataTestId={getNodeSetterId('dataset_use_rewrite')}
          />
        </div>
      )}

      {isShowGraph && (
        <div className={s['setting-item']}>
          <CheckboxWithLabel
            checked={useGraph}
            onChange={checked => {
              onDataSetInfoChange({
                ...dataSetInfo,
                useGraph: checked,
              });
            }}
            readonly={readonly}
            label={I18n.t('dataset_graph')}
            description={I18n.t('dataset_graph_hint')}
            dataTestId={getNodeSetterId('dataset_use_graph')}
          />
        </div>
      )}
    </div>
  );
};
