package wanwu_util

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	http_client "github.com/coze-dev/coze-studio/backend/pkg/http-client"
)

const (
	WanWuBuiltinSkillListUrlEnv  = "WANWU_CALLBACK_BUILTIN_SKILL_LIST_URL"
	WanWuCustomSkillListUrlEnv   = "WANWU_CALLBACK_CUSTOM_SKILL_LIST_URL"
	WanWuAcquiredSkillListUrlEnv = "WANWU_CALLBACK_ACQUIRED_SKILL_LIST_URL"
	SkillTypeBuiltin             = "builtin"
	SkillTypeCustom              = "custom"
	SkillTypeAcquired            = "acquired"
)

var supportedSkillTypes = []string{SkillTypeBuiltin, SkillTypeCustom, SkillTypeAcquired}

type SkillIdentity struct {
	SkillID   string
	SkillType string
}

type SkillToolInfo struct {
	SkillId    string `json:"skillId"`
	SkillType  string `json:"skillType"`
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Avatar     string `json:"avatar"`
	ObjectPath string `json:"objectPath"`
}

type skillListRequest struct {
	SkillIDList []string `json:"skillIdList"`
}

type skillListResponse struct {
	Code int64                 `json:"code"`
	Msg  string                `json:"msg"`
	Data skillListResponseData `json:"data"`
}

type skillListResponseData struct {
	SkillList []skillListItem `json:"skillList"`
}

type skillListItem struct {
	SkillID       string          `json:"skillId"`
	Name          string          `json:"name"`
	Author        string          `json:"author"`
	Desc          string          `json:"desc"`
	SkillPath     string          `json:"skillPath"`
	ObjectPath    string          `json:"objectPath"`
	SkillMarkdown string          `json:"skillMarkdown"`
	Avatar        skillAvatarInfo `json:"avatar"`
}

type skillAvatarInfo struct {
	Key  string `json:"key"`
	Path string `json:"path"`
}

func FetchSkillToolInfoList(ctx context.Context, skills []SkillIdentity) ([]*SkillToolInfo, error) {
	if len(skills) == 0 {
		return make([]*SkillToolInfo, 0), nil
	}

	groupedIDs := make(map[string][]string, len(supportedSkillTypes))
	for _, skillType := range supportedSkillTypes {
		groupedIDs[skillType] = make([]string, 0)
	}
	seen := make(map[string]struct{}, len(skills))

	for _, skill := range skills {
		skillID := strings.TrimSpace(skill.SkillID)
		skillType := strings.TrimSpace(skill.SkillType)
		if skillID == "" {
			return nil, fmt.Errorf("skillId is empty")
		}
		if !isSupportedSkillType(skillType) {
			return nil, fmt.Errorf("unsupported skillType %q for skillId %q", skillType, skillID)
		}

		key := buildSkillKey(skillType, skillID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		groupedIDs[skillType] = append(groupedIDs[skillType], skillID)
	}

	resultMap := make(map[string]*SkillToolInfo, len(seen))
	for _, skillType := range supportedSkillTypes {
		skillIDs := groupedIDs[skillType]
		if len(skillIDs) == 0 {
			continue
		}

		fetched, err := fetchSkillToolInfoMapByType(ctx, skillType, skillIDs)
		if err != nil {
			return nil, err
		}
		for key, info := range fetched {
			resultMap[key] = info
		}
	}

	result := make([]*SkillToolInfo, 0, len(skills))
	for _, skill := range skills {
		key := buildSkillKey(strings.TrimSpace(skill.SkillType), strings.TrimSpace(skill.SkillID))
		info, ok := resultMap[key]
		if !ok {
			return nil, fmt.Errorf("skill detail missing for skillType=%q skillId=%q", skill.SkillType, skill.SkillID)
		}
		result = append(result, cloneSkillToolInfo(info))
	}

	return result, nil
}

func fetchSkillToolInfoMapByType(ctx context.Context, skillType string, skillIDs []string) (map[string]*SkillToolInfo, error) {
	rawURL, err := skillListURL(skillType)
	if err != nil {
		return nil, err
	}

	reqBody := skillListRequest{SkillIDList: skillIDs}
	resp, err := http_client.GetRestyClientWithTimeout(time.Minute).R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(reqBody).
		Post(rawURL)
	if err != nil {
		return nil, fmt.Errorf("request %v err: %v", rawURL, err)
	}

	var respBody skillListResponse
	if len(resp.Body()) > 0 {
		if err := json.Unmarshal(resp.Body(), &respBody); err != nil {
			return nil, fmt.Errorf("request %v unmarshal response body: %v", rawURL, err)
		}
	}
	if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request %v http status %v msg: %v", rawURL, resp.StatusCode(), respBody.Msg)
	}
	if !isSuccessSkillListCode(respBody.Code) {
		return nil, fmt.Errorf("request %v business code %v msg: %v", rawURL, respBody.Code, respBody.Msg)
	}

	result := make(map[string]*SkillToolInfo, len(respBody.Data.SkillList))
	for _, item := range respBody.Data.SkillList {
		info, err := buildSkillToolInfo(skillType, item)
		if err != nil {
			return nil, err
		}
		result[buildSkillKey(skillType, info.SkillId)] = info
	}

	for _, skillID := range skillIDs {
		key := buildSkillKey(skillType, skillID)
		if _, ok := result[key]; !ok {
			return nil, fmt.Errorf("request %v missing skill detail for skillType=%q skillId=%q", rawURL, skillType, skillID)
		}
	}

	return result, nil
}

func buildSkillToolInfo(skillType string, item skillListItem) (*SkillToolInfo, error) {
	skillID := strings.TrimSpace(item.SkillID)
	if skillID == "" {
		return nil, fmt.Errorf("skill detail has empty skillId for skillType=%q", skillType)
	}

	objectPath := strings.TrimSpace(item.ObjectPath)
	if skillType == SkillTypeBuiltin {
		objectPath = strings.TrimSpace(item.SkillPath)
	}
	if objectPath == "" {
		return nil, fmt.Errorf("skill detail has empty object path for skillType=%q skillId=%q", skillType, skillID)
	}

	return &SkillToolInfo{
		SkillId:    skillID,
		SkillType:  skillType,
		Name:       item.Name,
		Desc:       item.Desc,
		Avatar:     strings.TrimSpace(item.Avatar.Key),
		ObjectPath: objectPath,
	}, nil
}

func skillListURL(skillType string) (string, error) {
	var envKey string
	switch skillType {
	case SkillTypeBuiltin:
		envKey = WanWuBuiltinSkillListUrlEnv
	case SkillTypeCustom:
		envKey = WanWuCustomSkillListUrlEnv
	case SkillTypeAcquired:
		envKey = WanWuAcquiredSkillListUrlEnv
	default:
		return "", fmt.Errorf("unsupported skillType %q", skillType)
	}

	rawURL := strings.TrimSpace(os.Getenv(envKey))
	if rawURL == "" {
		return "", fmt.Errorf("%s is empty", envKey)
	}
	return rawURL, nil
}

func buildSkillKey(skillType, skillID string) string {
	return skillType + ":" + skillID
}

func isSupportedSkillType(skillType string) bool {
	for _, supportedType := range supportedSkillTypes {
		if skillType == supportedType {
			return true
		}
	}
	return false
}

func isSuccessSkillListCode(code int64) bool {
	return code == 0 || code == 200
}

func cloneSkillToolInfo(info *SkillToolInfo) *SkillToolInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}
