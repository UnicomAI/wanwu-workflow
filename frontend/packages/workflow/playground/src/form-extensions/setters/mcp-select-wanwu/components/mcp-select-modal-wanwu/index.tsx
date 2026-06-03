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

/* eslint-disable max-lines */
import React, {
  useState,
  type FC,
  useRef,
  useEffect,
  type ReactNode,
} from 'react';

import classNames from 'classnames';
import { useInfiniteScroll } from 'ahooks';
import { IconSpin } from '@douyinfe/semi-icons';
import { I18n } from '@coze-arch/i18n';
import {
  UICompositionModal,
  UICompositionModalMain,
  UIEmpty,
} from '@coze-arch/bot-semi';
import {
  type McpInfo,
} from '@coze-arch/bot-api/memory';
import { MemoryApi } from '@coze-arch/bot-api';
import {
  Spin,
  Toast,
  Collapse
} from '@coze-arch/coze-design';
import { McpListItem } from './items';

import styles from './index.module.less';

interface SelectMcpModalProps {
  visible: boolean;
  onClose: () => void;
  onAddMcp: (id: string, addCallback?: () => void) => void;
  onRemoveMcp?: (id: string, removeCallback?: () => void) => void;
  onCreateDatabase?: (id: string, draftId: string) => void;
  enterFrom: string;
  botId?: string;
  workflowId?: string;
  spaceId: string;
  workflowAddList?: string[];
  projectID?: string;
  tips?: ReactNode;
}

interface GetMcpListData {
  list: McpInfo[];
  nextOffset: number;
  total: number;
  hasMore: boolean | undefined;
}

// eslint-disable-next-line @coze-arch/max-line-per-function, max-lines-per-function
export const useSelectMcpModal = ({
  visible,
  onClose,
  onAddMcp,
  onRemoveMcp,
  workflowAddList = [],
  tips,
}: SelectMcpModalProps) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [activeKey, setActiveKey] = useState<any>([])
  const [toolList, setToolList] = useState<any>([])
  const [toolLoading, setToolLoading] = useState<boolean>(false)

  const fetchMcpList = async (reqParams: {
    page_offset: number;
  }) => {
    const { page_offset } = reqParams;
    const { data }: { data: any } = await MemoryApi.GetMcpSelect();
    return {
      list: data?.list || [],
      nextOffset: page_offset + 1,
      total: data?.list?.length as number || 0,
      hasMore: false,
    };
  };

  const fetchMcpToolList = async (reqParams: {
    mcpId?: string;
    type?: string;
    serverUrl: string;
    transport?: string;
  }) => {
    try {
      setToolLoading(true)
      setToolList([])

      const { data }: { data: any } = await MemoryApi.GetMcpToolSelect(reqParams as any) || {};
      setToolList(data?.tools || [])
    } catch (err:any) {
      const { statusText, data } = err?.response || {};
      Toast.error(data?.msg || statusText || 'Server Error');
    } finally {
      setToolLoading(false)
    }
  };

  const { loading, data, loadingMore, reload } = useInfiniteScroll(
    (newData?: GetMcpListData): Promise<GetMcpListData> =>
      fetchMcpList({
        page_offset: newData?.nextOffset || 0,
      }),
    {
      manual: true,
      // true meas there is more data
      isNoMore: newData => true, // Boolean(!newData?.total || !newData.hasMore),
      reloadDeps: [],
      target: scrollRef,
    },
  );

  const handleAddMcp = (item) => {
    if (onAddMcp && item.mcpId) {
      onAddMcp?.(item, reload);
    }
  };

  const handleRemoveMcp = (item: McpInfo) => {
    /*if (onRemoveMcp && item.mcpId) {
      onRemoveMcp?.(item.mcpId, reload);
    }*/
    /*这里删除的是mcp工具下面的工具节点，mcpId是mcp工具的id，name是工具节点的唯一值,所以改为那么*/
    if (onRemoveMcp && item.name) {
      onRemoveMcp?.(item.name, reload);
    }
  };

  const handleAdd = () => {
    window.location.href = window.location.origin + '/aibase/mcp'
  };

  const handleChange = (value: any) => {
    const currentKey = value.length ? value[value.length - 1] : 0
    const currentObj: any = data?.list?.filter(item => item.mcpId === currentKey)?.[0] || {}
    const mcpId = currentObj.mcpId || ''
    // 根据传输协议类型选择正确的 URL
    const transport = currentObj.transport || ''
    let serverUrl = ''
    if (transport === 'streamable') {
      // streamable 协议优先使用 streamableUrl，fallback 到 serverUrl
      serverUrl = currentObj.streamableUrl || currentObj.serverUrl || ''
    } else {
      // 默认使用 serverUrl (sse url)
      serverUrl = currentObj.serverUrl || ''
    }

    setActiveKey([currentKey])
    // 优先传递 mcpId，后端会根据 mcpId 查询正确的 serverUrl 和 transport
    // 如果没有 mcpId，则传递 serverUrl 和 transport
    if (mcpId || serverUrl) {
       fetchMcpToolList({ mcpId, type: currentObj.type || '', serverUrl, transport })
    }
  }
  
  const renderList = () => (
    <div
      className="overflow-y-auto relative h-full"
      ref={scrollRef}
    >
      {/* FIXME: This needs to be rendered according to the actual situation. */}
      <Collapse
        // defaultActiveKey={data?.list.map(item => item.itemKey)}
        activeKey={activeKey}
        onChange={handleChange}
      >
        {data?.list.map((item:any, index) => (
          <Collapse.Panel
            header={item.name}
            itemKey={item.mcpId}
            key={`${index}mcp-collapse`}
          >
            <div className="w-[100%] max-h-[300px] overflow-y-auto">
              {toolList.map((it: any, index: number) => (
                <McpListItem
                  title={it.name}
                  description={it.description}
                  //当前toolList中不存在mcpId，所以不能根据it.mcpId判断是否添加,需要根据it.name判断是否添加过
                  isAdd={
                    Boolean(
                      // it.mcpId &&
                      item.mcpId &&
                      workflowAddList?.length &&
                      // workflowAddList?.includes(item.mcpId),
                      workflowAddList?.includes(it.name)
                    )
                  }
                  onAdd={() => handleAddMcp({
                    ...it,
                    apiAuth: item.apiAuth,
                    headers: item.headers,
                    serverUrl: item.serverUrl,
                    streamableUrl: item.streamableUrl,
                    transport: item.transport,
                    mcpId: item.mcpId,
                    id: item.mcpId,
                    mcpType: item.type
                  })}
                  // onRemove={() => handleRemoveMcp(item)}
                  //需要删除的是工具节点，所以根据it.name判断是否删除过
                  onRemove={() => handleRemoveMcp(it)}
                  key={item.name + index}
                />
              ))}
            </div>
          </Collapse.Panel>
        ))}
      </Collapse>
      {loadingMore ? (
        <div className={styles['loading-more']}>
          <IconSpin spin style={{ marginRight: '4px' }} />
          <div>{I18n.t('Loading')}</div>
        </div>
      ) : null}
    </div>
  );

  const renderEmpty = () => (
    <div className="overflow-y-auto relative w-full h-full flex justify-center items-center">
      <UIEmpty
        className="h-full"
        empty={{
          /*btnText: I18n.t('mcp_create_btn'),
          btnOnClick: handleAdd,*/
          title: I18n.t('mcp_empty'),
          description: I18n.t('mcp_empty_desc'),
        }}
      />
    </div>
  );

  const handleClose = () => {
    onClose();
  };


  useEffect(() => {
    if (visible) {
      reload();
    }
  }, [visible]);

  const renderContent = () => (
    <>
      {tips}
      <Spin
        spinning={loading || toolLoading}
        wrapperClassName={classNames(['overflow-hidden', styles.list])}
        style={{height: '100%'}}
      >
        {data?.list.length !== 0 ? renderList() : renderEmpty()}
      </Spin>
    </>
  );

  const renderMcp = () => (
    <React.Fragment>
      <UICompositionModal
        closable
        visible={visible}
        onCancel={handleClose}
        header={I18n.t('mcp_model_title')}
        sider={null}
        style={{width: '700px'}}
        content={
          <UICompositionModalMain className="relative px-[12px] gap-[16px]">
            {renderContent()}
          </UICompositionModalMain>
        }
      ></UICompositionModal>
    </React.Fragment>
  );

  return { renderMcp, renderContent };
};

export const SelectMcpModal: FC<SelectMcpModalProps> = props => {
  const { renderMcp } = useSelectMcpModal(props);

  return <>{renderMcp()}</>;
};
