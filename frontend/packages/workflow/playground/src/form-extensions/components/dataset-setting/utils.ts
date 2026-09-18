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

import { set } from 'lodash-es';

import { MatchType, type DataSetInfo } from './type';

/** Prompt beyond this value */
export const SUGGEST_TOP_K = 5;
/** default minimum match */
export const DEFAULT_MIN_SCORE = 0.4;

export const DEFAULT_SEMANTICS_PRIORITY = 0.2;
export const DEFAULT_KEYWORD_PRIORITY = 1;
export const DEFAULT_MAX_HISTORY = 0;
/** default maximum recall  */
export const DEFAULT_TOP_K = 5;
export const DEFAULT_MATCH_TYPE = MatchType.HybirdPriority;

export const getDefaultDatasetSetting = (
  selectDataSet: any[] = [],
): DataSetInfo => {
  const isAllExternalKnowledge = selectDataSet.every(item => item?.external);
  const isShowGraph = selectDataSet.some(item => item?.graphSwitch);

  const initDataSetInfo: DataSetInfo = {
    threshold: DEFAULT_MIN_SCORE,
    topK: DEFAULT_TOP_K,
    matchType: DEFAULT_MATCH_TYPE,
    rerankModelId: '',
    maxHistory: DEFAULT_MAX_HISTORY,
    rerankKeywordPriority: DEFAULT_KEYWORD_PRIORITY,
    semanticsPriority: DEFAULT_SEMANTICS_PRIORITY,
  };

  set(initDataSetInfo, 'rewrite', !isAllExternalKnowledge);
  set(initDataSetInfo, 'useGraph', isShowGraph);
  set(initDataSetInfo, 'rerankKeywordPrioritySwitch', false);

  return initDataSetInfo;
};
