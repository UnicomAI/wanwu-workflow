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

import React, { useState, useEffect } from 'react';

import { I18n } from '@coze-arch/i18n';
import { UICompositionModal, UICompositionModalMain } from '@coze-arch/bot-semi';
import {
  Button,
  Empty,
  Input,
  Select,
  Tooltip
} from '@coze-arch/coze-design';
import { Toast, UITable } from '@coze-arch/bot-semi';
import { IconInfo } from '@coze-arch/bot-icons';
import { IconCozTrashCan, IconCozEdit } from '@coze-arch/coze-design/icons';
import { IllustrationNoContent } from '@douyinfe/semi-illustrations';
import { KnowledgeApi } from '@coze-arch/bot-api';

const TIME = 'time'
const STRING = 'string'
const NUMBER = 'number'
const typeList = [
  { key: STRING, value: 'String' },
  { key: NUMBER, value: 'Number' },
  { key: TIME, value: 'Time' },
]

export const MetadataCreateModal = ({
  metaDataList = [],
  visible = false,
  knowledgeId = '',
  handleClose,
  reloadData,
}: {
  metaDataList: any[];
  visible: boolean;
  knowledgeId: string;
  handleClose: () => void;
  reloadData: () => void;
}) => {
  // metadata value list
  const [currentMetaDataList, setCurrentMetaDataList] = useState<any[]>([]);

  useEffect(() => {
    if (metaDataList?.length) {
      setCurrentMetaDataList(metaDataList);
    } else {
      setCurrentMetaDataList([]);
    }
  }, [metaDataList]);

  const setInputMetaDataValue = (index: number, v: any, key: string) => {
    const newMetaDataValueList = [...currentMetaDataList];
    if (newMetaDataValueList[index]) {
      newMetaDataValueList[index][key] = v;
      setCurrentMetaDataList(newMetaDataValueList);
    }
  }

  const formatUpdateItemValue = (item: any) => {
    return {
      metaId: item.metaId,
      metaKey: item.metaKey,
      option: item.option
    }
  }

  const submitMetaData = async (submitMetaDataList: any[], isNotClose: any) => {
    try {
      await KnowledgeApi.updateMetaSelectList({
        knowledgeId,
        metaDataList: submitMetaDataList,
      });
      Toast.success({
        content: I18n.t('dataset_metadata_operate_success'),
        showClose: false,
      });
      reloadData?.();
      if (!isNotClose) handleClose?.();
    } catch (err) {
      const { statusText, data } = err?.response || {};
      Toast.error(data?.msg || statusText || 'Server Error');
    }
  }

  // @ts-ignore
  return (
    <div>
      <UICompositionModal
        header={
          <div className="flex items-center">
            <div>{I18n.t('datasets_metadata_management')}</div>
          </div>
        }
        visible={visible}
        style={{width: '600px'}}
        centered
        onCancel={() => {
          handleClose?.()
        }}
        content={
          <UICompositionModalMain className="px-[12px]">
            <div className="h-full">
              <div
                className="overflow-y-auto"
                style={{ maxHeight: 'calc(100vh - 205px)' }}
              >
                {currentMetaDataList?.length > 0 ? (
                  <div className="flex items-center">
                    <UITable
                      useHoverStyle={false}
                      tableProps={{
                        dataSource: currentMetaDataList,
                        rowKey: 'metaId',
                        columns: [
                          {
                            title: (
                              <div className="flex items-center">
                                <div>Key</div>
                                <Tooltip
                                  showArrow
                                  position="top"
                                  style={{
                                    maxWidth: '300px',
                                    padding: '8px 12px',
                                    borderRadius: '6px',
                                  }}
                                  content={I18n.t('datasets_metadata_filter_key')}
                                >
                                  <IconInfo className="ml-[3px] cursor-pointer"/>
                                </Tooltip>
                              </div>
                            ),
                            dataIndex: 'metaKey',
                            width: 150,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <Input
                                  className="w-full"
                                  value={currentMetaDataList[index]?.metaKey || ''}
                                  disabled={!item.option}
                                  onChange={(v) => {
                                    setInputMetaDataValue(index, v, 'metaKey');
                                  }}
                                  onBlur={(e) => {
                                    const regex = /^[a-z][a-z0-9_]*$/;
                                    if (!regex.test(e?.target?.value)) {
                                      setInputMetaDataValue(index, '', 'metaKey');
                                      Toast.warning({
                                        content: I18n.t('dataset_metadata_key_value_error'),
                                        showClose: false,
                                      });
                                    }
                                  }}
                                />
                              );
                            },
                          },
                          {
                            title: I18n.t('datasets_metadata_type'),
                            dataIndex: 'metaValueType',
                            width: 80,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <Select
                                  className="w-[120px]"
                                  value={currentMetaDataList[index]?.metaValueType}
                                  disabled={item.option !== 'add'}
                                  onChange={(v: any) => {
                                    setInputMetaDataValue(index, v, 'metaValueType');
                                  }}
                                >
                                  {typeList.map((itemType: any) => (
                                    <Select.Option key={itemType.key} value={itemType.key}>
                                      {itemType.value}
                                    </Select.Option>
                                  ))}
                                </Select>
                              );
                            },

                          },
                          {
                            title: I18n.t('datasets_metadata_operation'),
                            align: 'center',
                            width: 50,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <>
                                  {/* v0.3.6: editing and deleting previous keys are prohibited */}
                                  {/*<IconCozEdit
                                    className="mr-[10px]"
                                    onClick={() => {
                                      const newCurrentMetaDataList = [...currentMetaDataList];
                                      if (!newCurrentMetaDataList[index].option) {
                                        newCurrentMetaDataList[index].option = 'update';
                                        setCurrentMetaDataList(newCurrentMetaDataList);
                                      }
                                    }}
                                  />*/}
                                  <IconCozTrashCan
                                    style={
                                      item.option !== 'add'
                                        ? { cursor: 'not-allowed', opacity: 0.5 }
                                        : {}
                                    }
                                    onClick={() => {
                                      if (item.option !== 'add') return
                                      const newCurrentMetaDataList = JSON.parse(JSON.stringify([...currentMetaDataList]));
                                      if (newCurrentMetaDataList[index]?.metaId) {
                                        const deleteData = [{
                                          metaId: newCurrentMetaDataList[index]?.metaId,
                                          option: 'delete'
                                        }];
                                        submitMetaData(deleteData, true);
                                      } else {
                                        newCurrentMetaDataList.splice(index, 1);
                                        setCurrentMetaDataList([...newCurrentMetaDataList]);
                                      }
                                    }}
                                  />
                                </>
                              );
                            },
                          },
                        ],
                      }}
                    />
                  </div>
                ) : (
                  <Empty
                    className="h-full justify-center mt-[50px] mb-[60px]"
                    image={<IllustrationNoContent className="w-[140px] h-[140px]" />}
                    title={I18n.t('variables_user_data_empty')}
                  />
                )}
                <div className="text-center mt-[20px] pb-[20px]">
                  <Button
                    color="brand"
                    className="mr-[10px]"
                    onClick={() => {
                      const newCurrentMetaDataList = [...currentMetaDataList];
                      newCurrentMetaDataList.push({
                        metaKey: '',
                        metaValueType: STRING,
                        option: 'add'
                      })
                      setCurrentMetaDataList(newCurrentMetaDataList);
                    }}
                  >
                    {I18n.t('datasets_metadata_create')}
                  </Button>
                  <Button
                    color="brand"
                    disabled={!currentMetaDataList?.length}
                    onClick={() => {
                      const submitMetaDataList = currentMetaDataList
                        .filter(item => item.option)
                        .map(item => item.option === 'update' ? formatUpdateItemValue(item) : item);
                      const hasNullKey = submitMetaDataList?.some((item:any) => !item.metaKey);

                      if (hasNullKey) {
                        Toast.warning({
                          content: I18n.t('dataset_metadata_key_error'),
                          showClose: false,
                        });
                        return
                      }
                      submitMetaData(submitMetaDataList, false);
                    }}
                  >
                    {I18n.t('datasets_metadata_confirm')}
                  </Button>
                </div>
              </div>
            </div>
          </UICompositionModalMain>
        }
      ></UICompositionModal>
    </div>
  );
};
