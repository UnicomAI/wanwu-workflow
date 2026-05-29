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

import React, { useEffect } from 'react';

import { useMemoizedFn } from 'ahooks';
import { FormPanelLayout } from '@coze-workflow/test-run';
import { USER_INPUT } from '@coze-workflow/base';
import { I18n } from '@coze-arch/i18n';
import { CreateEnv } from '@coze-arch/idl/workflow_api';
import {
  IntelligenceType,
  type IntelligenceBasicInfo,
} from '@coze-arch/idl/intelligence_api';
import { Empty } from "@coze-arch/coze-design";
import { IllustrationNoContent } from "@douyinfe/semi-illustrations";
import { type WorkflowNodeEntity } from '@/test-run-kit';
import { useGlobalState, useWorkflowRunService } from '@/hooks';
import { WorkflowExecStatus } from '@/entities';
import { useChatflowInfo } from '@/components/test-run/hooks/use-chatflow-info';

import { TestFormSheetHeaderWanwu } from '../test-form-sheet-v2/header-wanwu';
import { useTestFormSchema } from '../hooks/use-test-form-schema';
import {
  useGetStartNode,
  useGetStartNodeOutputs,
} from '../hooks/use-get-start-node';
import { FieldName } from '../constants';
import { ChatHistory } from '../chat-history';
import {
  ChatFlowTestFormProvider,
  useChatFlowTestFormStore,
} from './test-form-provider';
import { TestFormFloatButton } from './test-form-float-button';
import { ChatFlowTestForm } from './test-form';
import { ConversationSelectWanwu } from './conversation-select-wanwu';

import css from './test-form-panel-wanwu.module.less';

const EmptyUI = ({description}) => (
  <div className={css['test-form-right-empty']}>
    <Empty
      image={
        <IllustrationNoContent style={{width: 112, height: 112}}/>
      }
      description={description}
    />
  </div>
);

export interface ChatFlowTestFormPanelPropsWanwu {
  node: WorkflowNodeEntity;
}

const ChatFlowTestRunHistory = (props: {
  projectInfo?: IntelligenceBasicInfo;
  showInputArea?: boolean;
}) => {
  const {formData} = useChatFlowTestFormStore(store => ({
    formData: store.formData,
  }));
  const { projectInfo, ...restProps } = props;
  const { config } = useGlobalState();
  const { getNode } = useGetStartNode();
  const { getStartNodeOutputs } = useGetStartNodeOutputs();

  const startNode = getNode();
  let defaultText = '';
  if (startNode) {
    const outputs = getStartNodeOutputs();
    defaultText =
      outputs.find(output => output.name === USER_INPUT)?.defaultValue || '';
  }
  const runService = useWorkflowRunService();
  const { sessionInfo, conversationInfo } = useChatflowInfo();
  const inputData = formData?.[FieldName.Node]?.[FieldName.Input];
  const projectOrBotInfo = sessionInfo
    ? {
        id: sessionInfo.value,
        name: sessionInfo.name,
        iconUrl: sessionInfo.avatar,
        type: sessionInfo.type,
      }
    : {
        id: projectInfo?.id || '',
        name: projectInfo?.name || '',
        iconUrl: projectInfo?.icon_url || '',
        type: IntelligenceType.Project,
      };

  // Get the default value of the start node
  return projectOrBotInfo?.id ? (
    conversationInfo ? (
      <div className={css['chat-history-content']}>
        <ChatHistory
          type={CreateEnv.Draft}
          projectOrBotInfo={projectOrBotInfo}
          workflowInfo={{
            id: config.workflowId,
            parameters: inputData,
            header: {
              'rpc-persist-mock-traffic-enable': '1',
            },
          }}
          activateChat={{
            unique_id: conversationInfo?.value,
            conversation_name:
              projectOrBotInfo?.type === IntelligenceType.Bot
                ? projectOrBotInfo?.name
                : conversationInfo?.label,
            conversation_id: conversationInfo?.conversationId,
          }}
          onGetChatFlowExecuteId={(executeId: string) => {
            // Help backend @zhangshiqi.live compatibility logic
            // Do not use the newly given executeId when there is already a polling in progress
            if (
              runService.globalState.viewStatus === WorkflowExecStatus.EXECUTING
            ) {
              return;
            }
            runService.clearTestRun();
            runService.getRTProcessResult({executeId});
          }}
          topSlot={(isChatError?: boolean) => (
            null // <TestFormFloatButton isChatError={isChatError}/>
          )}
          defaultText={defaultText}
          {...restProps}
        />
      </div>
    ) : (
      <EmptyUI description={I18n.t('workflow_testrun_chatflow_add_conversation_hint')} />
    )
  ) : (
    <EmptyUI description={I18n.t('workflow_testrun_chatflow_desc')} />
  );
};

const ChatflowFormPanel = ({ node }: ChatFlowTestFormPanelPropsWanwu) => {
  const { visible, patch } = useChatFlowTestFormStore(store => ({
    visible: store.visible,
    patch: store.patch,
  }));
  const { generate } = useTestFormSchema();
  const init = useMemoizedFn(async () => {
    const schema = await generate();
    if (schema?.fields.length) {
      patch({ visible: true, hasForm: true });
    }
  });
  useEffect(() => {
    // Open by default
    init();
  }, []);
  return (
    <div className={visible ? '' : css['hide-chatflow-form']}>
      <ChatFlowTestForm node={node} />
    </div>
  )
};

export const ChatFlowTestFormPanelWanwu: React.FC<ChatFlowTestFormPanelPropsWanwu> = ({
  node,
}) => {
  const { getProjectApi } = useGlobalState();
  const projectInfo = getProjectApi()?.ideGlobalStore(
    store => store.projectInfo?.projectInfo,
  );

  return (
    <ChatFlowTestFormProvider>
      <div className={css['test-form-content-wanwu']}>
        <div className={css['test-form-left']}>
          <FormPanelLayout className={css['test-form-wanwu']}>
            <TestFormSheetHeaderWanwu />
            <ConversationSelectWanwu />
            <ChatflowFormPanel node={node} />
          </FormPanelLayout>
        </div>
        <div className={css['test-form-right']}>
          <ChatFlowTestRunHistory
            projectInfo={projectInfo}
            showInputArea={true}
          />
        </div>
      </div>
    </ChatFlowTestFormProvider>
  );
};
