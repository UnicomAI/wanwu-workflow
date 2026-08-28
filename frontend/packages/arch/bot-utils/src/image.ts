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

export const loadImage = (url: string): Promise<void> =>
  new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve();
    img.onerror = reject;
    img.src = url;
  });

// avatar.path from backend is a relative path that must be served under /user/api.
// Pass through absolute URLs unchanged; strip leading slashes so it stays idempotent.
export const getAvatarUrl = (path?: string): string => {
  if (!path) {
    return '';
  }
  if (/^https?:\/\//i.test(path) || path.startsWith('/user/api')) {
    return path;
  }
  return `/user/api/${path.replace(/^\/+/, '')}`;
};
