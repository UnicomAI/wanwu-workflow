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
import dayjs from 'dayjs';

import { I18n } from '@coze-arch/i18n';
import { useNodeTestId } from '@coze-workflow/base';
import { UICompositionModal, UICompositionModalMain } from '@coze-arch/bot-semi';
import {
  Button,
  CozInputNumber,
  DatePicker,
  Empty,
  Input,
  Select,
  Tooltip
} from '@coze-arch/coze-design';
import { Checkbox, Toast, UITable } from '@coze-arch/bot-semi';
import { IconInfo } from '@coze-arch/bot-icons';
import { IconCozPlus, IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { IllustrationNoContent } from '@douyinfe/semi-illustrations';
import { KnowledgeApi } from '@coze-arch/bot-api';
import { MetadataCreateModal } from './metadata-create-modal-wanwu';

import style from './index.module.less';

interface ValueProps {
  dataset_id?: string,
  name?: string
}

const filterLogicTypeList = [
  {key: 'and', value: I18n.t('datasets_metadata_and')},
  {key: 'or', value: I18n.t('datasets_metadata_or')}
]

const TIME = 'time'
const STRING = 'string'
const NUMBER = 'number'
const EMPTY_CONDITIONS = ['is empty', 'is not empty']
const conditionList = {
  [TIME]: [
    {key: 'is', value: I18n.t('datasets_metadata_true')},
    {key: 'before', value: I18n.t('datasets_metadata_easy')},
    {key: 'after', value: I18n.t('datasets_metadata_later')},
    {key: 'is empty', value: I18n.t('datasets_metadata_null')},
    {key: 'is not empty', value: I18n.t('datasets_metadata_notNull')},
  ],
  [STRING]: [
    {key: 'is', value: I18n.t('datasets_metadata_true')},
    {key: 'is not', value: I18n.t('datasets_metadata_false')},
    {key: 'contains', value: I18n.t('datasets_metadata_include')},
    {key: 'not contains', value: I18n.t('datasets_metadata_notInclude')},
    {key: 'starts with', value: I18n.t('datasets_metadata_begin')},
    {key: 'ends with', value: I18n.t('datasets_metadata_end')},
    {key: 'is empty', value: I18n.t('datasets_metadata_null')},
    {key: 'is not empty', value: I18n.t('datasets_metadata_notNull')},
  ],
  [NUMBER]: [
    {key: '=', value: I18n.t('datasets_metadata_equal')},
    {key: '≠', value: I18n.t('datasets_metadata_notEqual')},
    {key: '>', value: I18n.t('datasets_metadata_greater')},
    {key: '≥', value: I18n.t('datasets_metadata_greaterEqual')},
    {key: '<', value: I18n.t('datasets_metadata_less')},
    {key: '≤', value: I18n.t('datasets_metadata_lessEqual')},
    {key: 'is empty', value: I18n.t('datasets_metadata_null')},
    {key: 'is not empty', value: I18n.t('datasets_metadata_notNull')},
  ],
}

export const DEFAULT_METADATA:any = {
  filterEnable: false,
  filterLogicType: 'and',
  metaFilterParams: []
}

export const MetadataFilterModal = ({
  defaultValue,
  visible = false,
  onSubmit,
  handleClose,
  knowledgeId = '',
}: {
  defaultValue: any;
  visible: boolean;
  handleClose: () => void;
  onSubmit: (value: ValueProps[]) => void;
  knowledgeId: string;
}) => {
  const { getNodeSetterId } = useNodeTestId();

  // metadata total data
  const [currentMetaData, setCurrentMetaData] = useState<any>(DEFAULT_METADATA);
  // metadata table value list, resolve the issue of constantly refreshing the table, input value flashing
  const [metaDataValueList, setMetaDataValueList] = useState<any[]>([]);
  // create metadata modal visible
  const [createVisible, setCreateVisible] = useState<boolean>(false);
  // current key list -> format metadata list for workflow
  const [currentKeyList, setCurrentKeyList] = useState<any[]>([]);
  // metadata list
  const [metaDataList, setMetaDataList] = useState<any[]>([]);

  const formatMetaDataValue = () => {
    // merge metaDataValueList to currentMetaData
    const metaFilterParams = JSON.parse(JSON.stringify(currentMetaData.metaFilterParams ?? []))
    const newMetaFilterParams = metaFilterParams.map((item, index) => ({...item, value: metaDataValueList[index]}))
    const newCurrentMetaData = {...currentMetaData, metaFilterParams: newMetaFilterParams}
    setCurrentMetaData(newCurrentMetaData)
    return newCurrentMetaData
  }

  const setInputMetaDataValue = (index: number, v: any) => {
    const newMetaDataValueList = [...metaDataValueList]
    newMetaDataValueList[index] = v
    setMetaDataValueList(newMetaDataValueList)
  }

  const formatMetaFilterParams = (data) => {
    if (!(data && data.length)) return []
    return data.map((item, index) => ({...item, id: `metaFilterParams_${index}`}))
  }

  const getMetaDataList = async () => {
    try {
      const { data } = await KnowledgeApi.getMetaSelectList({ knowledgeId });
      const knowledgeMetaList = data?.knowledgeMetaList || [];
      const newMetaDataList = knowledgeMetaList
        .map((item: any) => ({ key: item.metaKey, type: item.metaValueType }))
        .filter((item: any) => item.key);

      setMetaDataList(knowledgeMetaList);
      setCurrentKeyList(newMetaDataList);
    } catch (err) {
      const { statusText, data } = err?.response || {};
      Toast.error(data?.msg || statusText || 'Server Error');
    }
  }

  useEffect(() => {
    if (visible && knowledgeId) getMetaDataList()
  }, [knowledgeId, visible]);

  useEffect(() => {
    if (visible && defaultValue) {
      setCurrentMetaData(defaultValue)

      // set metadata value list, resolve input value flicker issue
      const { metaFilterParams } = defaultValue || {}
      setMetaDataValueList(metaFilterParams?.map(item => item.value) || [])
    }
  }, [defaultValue, visible]);

  return (
    <div>
      <UICompositionModal
        header={
          <div className="flex items-center">
            <div>{I18n.t('datasets_metadata_filter')}</div>
            <Tooltip
              showArrow
              position="top"
              style={{
                maxWidth: '380px',
                padding: '8px 12px',
                borderRadius: '6px',
              }}
              content={I18n.t('datasets_metadata_filter_desc')}
            >
              <IconInfo className="ml-[3px] cursor-pointer"/>
            </Tooltip>
            <Checkbox
              className="ml-[20px] mr-[3px]"
              checked={currentMetaData.filterEnable}
              onChange={e => {
                setCurrentMetaData({...currentMetaData, filterEnable: e?.target?.checked})
              }}
              data-testid={getNodeSetterId('use-dataset-metadata-filter')}
            />
            <div style={{fontWeight: 400, fontSize: '13px', color: '#888'}}>
              {I18n.t('datasets_metadata_filter_use')}
            </div>
          </div>
        }
        visible={visible}
        style={{width: '820px'}}
        centered
        onCancel={() => {
          setCurrentMetaData(DEFAULT_METADATA)
          handleClose?.()
        }}
        content={
          <UICompositionModalMain className="px-[12px]">
            <div className="h-full">
              <Button
                icon={<IconCozPlus />}
                color="primary"
                onClick={() => {
                  if (!currentMetaData.filterEnable) {
                    Toast.warning({
                      content: I18n.t('datasets_metadata_enable_hint'),
                      showClose: false,
                    });
                    return
                  }
                  // remove the judgment for adding new conditions, metadata can be added to the metadata list
                  /*if (!currentKeyList?.length) {
                    Toast.warning({
                      content: I18n.t('datasets_metadata_no_data_hint'),
                      showClose: false,
                    });
                    return
                  }*/
                  const metaFilterParams = JSON.parse(JSON.stringify(currentMetaData.metaFilterParams ?? []))
                  const newMetaFilterItem = {
                    condition: conditionList[currentKeyList[0]?.type || STRING]?.[0]?.key || '',
                    key: currentKeyList[0]?.key || '',
                    type: currentKeyList[0]?.type || STRING,
                    value: ""
                  }
                  const newMetaFilterParams = [...metaFilterParams, newMetaFilterItem]
                  setCurrentMetaData({...currentMetaData, metaFilterParams: newMetaFilterParams})
                }}
              >
                {I18n.t('Add_condition')}
              </Button>
              {currentMetaData?.metaFilterParams?.length > 0 ? (
                <div
                  className="mt-[20px] overflow-y-auto"
                  style={{ maxHeight: 'calc(100vh - 258px)' }}
                >
                  <div className="flex items-center">
                    <div className="mr-[10px]">
                      <Select
                        value={currentMetaData.filterLogicType}
                        onChange={(v: any) => {
                          setCurrentMetaData({...currentMetaData, filterLogicType: v})
                        }}
                      >
                        {filterLogicTypeList.map((item) => (
                          <Select.Option key={item.key} value={item.key}>
                            {item.value}
                          </Select.Option>
                        ))}
                      </Select>
                    </div>
                    <UITable
                      useHoverStyle={false}
                      tableProps={{
                        dataSource: formatMetaFilterParams(currentMetaData.metaFilterParams),
                        rowKey: 'id',
                        columns: [
                          {
                            title: "Key",
                            dataIndex: 'key',
                            width: 120,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <Select
                                  className="w-[100px]"
                                  value={value}
                                  onChange={(v: any) => {
                                    if (v === 'add') {
                                      setCreateVisible(true)
                                      return
                                    }
                                    // key change -> change type\condition\value
                                    const metaFilterParams = JSON.parse(JSON.stringify(currentMetaData.metaFilterParams ?? []))
                                    const keyObj = currentKeyList.find(item => item.key === v) || {}
                                    metaFilterParams[index].key = v
                                    metaFilterParams[index].type = keyObj.type || STRING
                                    metaFilterParams[index].condition = conditionList[keyObj.type || STRING]?.[0]?.key || ''
                                    setInputMetaDataValue(index, '')
                                    setCurrentMetaData({...currentMetaData, metaFilterParams})
                                  }}
                                >
                                  <Select.Option key="add" value="add" className={style['add-metadata']}>
                                    {`+ ${I18n.t('Add_metadata')}`}
                                  </Select.Option>
                                  {currentKeyList.map((itemKey: any) => (
                                    <Select.Option key={itemKey.key + index} value={itemKey.key}>
                                      {itemKey.key}
                                    </Select.Option>
                                  ))}
                                </Select>
                              );
                            },
                          },
                          {
                            title: I18n.t('datasets_metadata_type'),
                            dataIndex: 'type',
                            width: 90,
                          },
                          {
                            title: I18n.t('datasets_metadata_condition'),
                            dataIndex: 'condition',
                            width: 110,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <Select
                                  className="w-[80px]"
                                  value={value}
                                  onChange={(v: any) => {
                                    const metaFilterParams = JSON.parse(JSON.stringify(currentMetaData.metaFilterParams ?? []))
                                    metaFilterParams[index].condition = v
                                    if (EMPTY_CONDITIONS.includes(v)) {
                                      setInputMetaDataValue(index, '')
                                    }
                                    setCurrentMetaData({...currentMetaData, metaFilterParams})
                                  }}
                                >
                                  {conditionList[item.type || STRING].map((itemType: any) => (
                                    <Select.Option key={itemType.key + index} value={itemType.key}>
                                      {itemType.value}
                                    </Select.Option>
                                  ))}
                                </Select>
                              );
                            },
                          },
                          {
                            title: 'Value',
                            dataIndex: 'value',
                            render: (value: string, item: any, index: number) => {
                              const isValueDisabled = EMPTY_CONDITIONS.includes(item.condition)
                              return (
                                item.type === TIME ? (
                                  <DatePicker
                                    disabled={isValueDisabled}
                                    className="w-full"
                                    type="dateTime"
                                    value={metaDataValueList[index] ? Number(metaDataValueList[index]) : ''}
                                    format="yyyy-MM-dd HH:mm:ss"
                                    onChange={(date, dateString) => {
                                      if (typeof dateString === 'string') {
                                        setInputMetaDataValue(index, String(dayjs(dateString).valueOf()))
                                      }
                                    }}
                                    inputStyle={{ width: '100%' }}
                                  />
                                ) : (
                                  item.type === NUMBER ? (
                                    <CozInputNumber
                                      disabled={isValueDisabled}
                                      className="w-full"
                                      value={metaDataValueList[index]}
                                      onChange={v => {
                                        setInputMetaDataValue(index, String(v))
                                      }}
                                    />
                                  ) : (
                                    <Input
                                      disabled={isValueDisabled}
                                      className="w-full"
                                      value={metaDataValueList[index]}
                                      onChange={(v) => {
                                        setInputMetaDataValue(index, v)
                                      }}
                                    />
                                  )
                                )
                              );
                            },
                          },
                          {
                            title: I18n.t('datasets_metadata_operation'),
                            align: 'center',
                            width: 90,
                            render: (value: string, item: any, index: number) => {
                              return (
                                <IconCozTrashCan
                                  onClick={() => {
                                    // delete current value object
                                    const metaFilterParams = JSON.parse(JSON.stringify(currentMetaData.metaFilterParams ?? []))
                                    metaFilterParams.splice(index, 1)
                                    setCurrentMetaData({...currentMetaData, metaFilterParams})
                                    // delete current value
                                    const newMetaDataValueList = [...metaDataValueList]
                                    newMetaDataValueList.splice(index, 1)
                                    setMetaDataValueList(newMetaDataValueList)
                                    
                                    // 新增：如果删除后没有过滤条件了，自动提交空数组
                                    if (!metaFilterParams?.length) {
                                      onSubmit?.(DEFAULT_METADATA)
                                    }
                                  }}
                                />
                              );
                            },
                          },
                        ],
                      }}
                    />
                  </div>
                  <div className="text-center mt-[20px] pb-[20px]">
                    <Button
                      color="brand"
                      onClick={() => {
                        const metaDataFilterParams = formatMetaDataValue()
                        const hasNullValue = metaDataFilterParams.metaFilterParams?.some(item => {
                          if (EMPTY_CONDITIONS.includes(item.condition)) {
                            return !item.key
                          }
                          return !(item.key && item.value)
                        })
                        if (hasNullValue) {
                          Toast.warning({
                            content: I18n.t('datasets_metadata_filter_error'),
                            showClose: false,
                          })
                          return
                        }
                        onSubmit?.(metaDataFilterParams)
                      }}
                    >
                      {I18n.t('datasets_metadata_confirm')}
                    </Button>
                  </div>
                </div>
              ) : (
                <Empty
                  className="h-full justify-center mt-[-50px]"
                  image={<IllustrationNoContent className="w-[140px] h-[140px]" />}
                  title={I18n.t('variables_user_data_empty')}
                />
              )}
            </div>
          </UICompositionModalMain>
        }
      ></UICompositionModal>
      <MetadataCreateModal
        visible={createVisible}
        handleClose={() => setCreateVisible(false)}
        metaDataList={metaDataList}
        knowledgeId={knowledgeId}
        reloadData={getMetaDataList}
      />
    </div>
  );
};
