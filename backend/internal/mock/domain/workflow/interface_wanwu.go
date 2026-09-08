package mockWorkflow

import (
	context "context"
	reflect "reflect"

	vo "github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	gomock "go.uber.org/mock/gomock"
)

// GetVersionList mocks base method.
func (m *MockRepository) GetVersionListByWanwu(ctx context.Context, id int64) ([]*vo.VersionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVersionListByWanwu", ctx, id)
	ret0, _ := ret[0].([]*vo.VersionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVersionList indicates an expected call of GetVersionList.
func (mr *MockRepositoryMockRecorder) GetVersionListByWanwu(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVersionListByWanwu", reflect.TypeOf((*MockRepository)(nil).GetVersionListByWanwu), ctx, id)
}

// UpdateWorkflowVersionDescription mocks base method.
func (m *MockRepository) UpdateWorkflowVersionDescriptionByWanwu(ctx context.Context, id int64, versionDescription string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateWorkflowVersionDescriptionByWanwu", ctx, id, versionDescription)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateWorkflowVersionDescription indicates an expected call of UpdateWorkflowVersionDescription.
func (mr *MockRepositoryMockRecorder) UpdateWorkflowVersionDescriptionByWanwu(ctx, id, versionDescription any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(
		mr.mock,
		"UpdateWorkflowVersionDescriptionByWanwu",
		reflect.TypeOf((*MockRepository)(nil).UpdateWorkflowVersionDescriptionByWanwu),
		ctx, id, versionDescription,
	)
}

// MGetWorkflowLatestVersionByWanwu mocks base method.
func (m *MockRepository) MGetWorkflowLatestVersionByWanwu(ctx context.Context, workflowIDs []int64) (map[int64]*vo.VersionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MGetWorkflowLatestVersionByWanwu", ctx, workflowIDs)
	ret0, _ := ret[0].(map[int64]*vo.VersionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MGetWorkflowLatestVersionByWanwu indicates an expected call of MGetWorkflowLatestVersionByWanwu.
func (mr *MockRepositoryMockRecorder) MGetWorkflowLatestVersionByWanwu(ctx, workflowIDs any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MGetWorkflowLatestVersionByWanwu", reflect.TypeOf((*MockRepository)(nil).MGetWorkflowLatestVersionByWanwu), ctx, workflowIDs)
}
