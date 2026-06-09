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
import {
  ValueExpressionType,
  type RefExpression,
  ViewVariableType,
  type InputValueVO,
  useNodeTestId,
} from '@coze-workflow/base';
import { I18n } from '@coze-arch/i18n';
import { Select } from "@coze-arch/coze-design";
import { DEFAULT_PARAMS_LIST } from '../constants';

import { ColumnsTitle } from '@/form-extensions/components/columns-title';
import { ValueExpressionInputField } from '@/node-registries/common/fields';
import {
  Section,
  useFieldArray,
  FieldArrayList,
  withFieldArray,
} from '@/form';

interface FileGenerateParamsFieldProps {
  disabledTypes?: ViewVariableType[];
  defaultValue?: RefExpression;
  name?: string;
  params?: any;
}

export const FileGenerateParamsField = withFieldArray(({
  disabledTypes,
}: FileGenerateParamsFieldProps) => {
  const { value, remove, append } = useFieldArray<InputValueVO>();
  const { getNodeSetterId } = useNodeTestId();
  const FILE_TYPE = 'fileType'

  const removeAll = () => {
    const valueArr = JSON.parse(JSON.stringify(value || []))
    valueArr.forEach(() => {
      remove(0)
    })
  }

  const appendValue = (appendList: any) => {
    appendList?.forEach((item: any) => {
      append(item)
    })
  }

  const formatFileType = (v: any) => ({
    name: FILE_TYPE,
    input: { content: v, type: ValueExpressionType.LITERAL },
  })

  const initAppend = () => {
    // 进入更新数据
    const appendList = DEFAULT_PARAMS_LIST.map(item => {
      if (value?.map(it => it.name).includes(item.name)) {
        return value[value.findIndex(it => it.name === item.name)];
      } else {
        return item.name === FILE_TYPE ? formatFileType('txt') : item
      }
    })
    removeAll()
    appendValue(appendList)
  }

  useEffect(() => {
    initAppend()
  }, []);

  return (
    <Section
      title={I18n.t('workflow_detail_node_input')}
      tooltip={I18n.t(
        'node_http_request_params_desc',
        {},
        '输入参数值',
      )}
    >
      <ColumnsTitle
        columns={[
          {
            title: I18n.t('workflow_detail_node_parameter_name'),
            style: {flex: 2},
          },
          {
            title: I18n.t('workflow_detail_end_output_value'),
            style: {flex: 3},
          },
        ]}
        className="mb-[8px]"
      />
      <FieldArrayList>
        {value?.map(({name, input}, index) => (
          name === FILE_TYPE ? (
            <div key={FILE_TYPE + index} className="flex gap-[4px] min-w-0">
              <span className="text-[12px] items-center gap-[4px] w-[152px]">fileType</span>
              <Select
                className="last:flex-1 min-w-0"
                size="small"
                value={input.content}
                onChange={(v: unknown) => {
                  const fileTypeIndex = value?.findIndex(item => item.name === FILE_TYPE)
                  if (fileTypeIndex !== -1) remove(fileTypeIndex)

                  append(formatFileType(v))
                }}
                data-testid={getNodeSetterId('getNodeSetterId-file-generate-fileType')}
              >
                {['txt', 'docx', 'pdf', 'md', 'html'].map((v, i) => (
                  <Select.Option
                    value={v}
                    key={v + i}
                    data-testid={getNodeSetterId('file-generate-fileType-option')}
                  >
                    {v}
                  </Select.Option>
                ))}
              </Select>
            </div>
          ) : (
            <ValueExpressionInputField
              key={name + index}
              label={name}
              required={true}
              inputType={ViewVariableType.String}
              disabledTypes={disabledTypes}
              name={`inputs.inputParameters.${index}.input`}
            />
          )
        ))}
      </FieldArrayList>
    </Section>
  )
});
