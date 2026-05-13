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

import { type FC } from 'react';

import { I18n } from '@coze-arch/i18n';
import { UICompositionModalSider, UIInput } from '@coze-arch/bot-semi';
import { IconSearch } from '@douyinfe/semi-icons';
import { type SkillType } from './types';

export interface SkillSelectSiderProps {
  activeTab: SkillType;
  searchKeyword: string;
  onTabChange: (key: SkillType) => void;
  onSearchChange: (keyword: string) => void;
  searchPlaceholder?: string;
}

const tabList = [
  /*{
    key: 'all',
    label: I18n.t('workflow_skill_tab_all' as any, {}, '全部'),
  },*/
  {
    key: 'builtin',
    label: I18n.t('builtin_skills_wanwu', {}, '内置'),
  },
  {
    key: 'custom',
    label: I18n.t('custom_skills_wanwu' as any, {}, '我创建的'),
  },
];

export const SkillSelectSider: FC<SkillSelectSiderProps> = ({
  activeTab,
  searchKeyword,
  onTabChange,
  onSearchChange,
  searchPlaceholder = I18n.t('Search', {}, '搜索'),
}) => {
  return (
    <UICompositionModalSider style={{ paddingTop: 16 }}>
      <UICompositionModalSider.Header>
        <UIInput
          placeholder={searchPlaceholder}
          value={searchKeyword}
          onChange={onSearchChange}
          className="w-full"
          showClear
          prefix={
            <IconSearch />
          }
        />
      </UICompositionModalSider.Header>

      <UICompositionModalSider.Content style={{ paddingTop: 16 }}>
        {tabList.map(item => {
          const isActive = item.key === activeTab;
          return (
            <div
              key={item.key}
              className={
                isActive
                  ? 'px-[12px] py-[10px] rounded-[8px] text-[14px] font-semibold mb-[8px] bg-[rgba(46,47,56,0.05)] text-[var(--Text-coz-text-primary,#1d2129)] cursor-pointer'
                  : 'px-[12px] py-[10px] rounded-[8px] text-[14px] mb-[8px] text-[var(--Text-coz-text-primary,#1d2129)] cursor-pointer'
              }
              onClick={() => {
                onTabChange(item.key as SkillType);
              }}
            >
              {item.label}
            </div>
          );
        })}
      </UICompositionModalSider.Content>
    </UICompositionModalSider>
  );
};
