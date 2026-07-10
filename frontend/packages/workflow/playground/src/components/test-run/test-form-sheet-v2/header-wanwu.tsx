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

import cls from 'classnames';
import { I18n } from '@coze-arch/i18n';
import { IconCozArrowLeft } from '@coze-arch/coze-design/icons';
import { IconButton } from "@coze-arch/coze-design";
import { getWorkflowHeaderTestId } from "../../workflow-header/utils";
import { useGlobalState } from "../../../hooks";

import styles from './index-wanwu.module.less';

export const TestFormSheetHeaderWanwu = () => {
  const globalState = useGlobalState();
  const { playgroundProps, info } = globalState;

  return (
    <div className={cls(styles['test-form-sheet-header-v2'], styles['test-form-sheet-header-v2-wanwu'])}>
      {/*<IconButton
        icon={<IconCozArrowLeft />}
        color="secondary"
        data-testid={getWorkflowHeaderTestId('back')}
        onClick={() => {
          playgroundProps.onBackClick?.(globalState);
        }}
      />*/}
      <div className={cls(styles['header-title-v2'])}>
        {info.name || ''} {/*{I18n.t('workflow_detail_title_testrun')}*/}
      </div>
    </div>
  );
};
