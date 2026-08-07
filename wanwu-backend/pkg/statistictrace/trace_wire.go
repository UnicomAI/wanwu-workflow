package statistictrace

import "google.golang.org/protobuf/encoding/protowire"

// traceRecord 本地字段定义；写入 Redis 时按 BFF common.TraceInfo 的 field number 编码。
type traceRecord struct {
	TraceID string
	APIPath string
	UserID  string
	OrgID   string
	AppID   string
	AppType string
	Extra   map[string]string
}

// appendStr 追加一个 protowire bytes 字段，为空时跳过。
func appendStr(b []byte, num protowire.Number, s string) []byte {
	if s == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendString(b, s)
}

// appendSubMsg2 追加一个嵌套消息，含两个字符串字段（field 1、field 2），均为空时跳过。
func appendSubMsg2(b []byte, tag protowire.Number, s1, s2 string) []byte {
	if s1 == "" && s2 == "" {
		return b
	}
	sub := appendStr(appendStr(nil, 1, s1), 2, s2)
	b = protowire.AppendTag(b, tag, protowire.BytesType)
	return protowire.AppendBytes(b, sub)
}

// forEachField 遍历 protowire 编码的顶层字段，对每个 bytes 字段调用 fn(num, value)。
// 非 bytes 字段或解析异常时跳过；统一容错解析逻辑，避免 decode 函数重复手写循环。
func forEachField(data []byte, fn func(num protowire.Number, val []byte)) {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return
		}
		data = data[n:]
		if typ != protowire.BytesType {
			if n = protowire.ConsumeFieldValue(num, typ, data); n < 0 {
				return
			}
			data = data[n:]
			continue
		}
		v, m := protowire.ConsumeBytes(data)
		if m < 0 {
			return
		}
		data = data[m:]
		fn(num, v)
	}
}

// decodeSubMsg2 解析含两个字符串字段（field 1、2）的嵌套消息，返回其值（统计数据，容错即可）。
func decodeSubMsg2(data []byte) (s1, s2 string) {
	forEachField(data, func(num protowire.Number, val []byte) {
		switch num {
		case 1:
			s1 = string(val)
		case 2:
			s2 = string(val)
		}
	})
	return s1, s2
}

// encodeTraceRecord 将 traceRecord 按 BFF common.TraceInfo 的 field number 序列化。
// TraceApi(2): apiPath=2；TraceUser(4): orgId=1, userId=2；TraceApp(5): appId=1, appType=2；
// traceExtra(6): key=1, value=2（map entry 展开为重复字段）。
func encodeTraceRecord(rec traceRecord) []byte {
	var b []byte
	b = appendStr(b, 1, rec.TraceID)
	b = appendSubMsg2(b, 2, "", rec.APIPath)
	b = appendSubMsg2(b, 4, rec.OrgID, rec.UserID)
	b = appendSubMsg2(b, 5, rec.AppID, rec.AppType)
	for k, v := range rec.Extra {
		b = appendSubMsg2(b, 6, k, v)
	}
	return b
}

// decodeTraceRecord 尽力解析 BFF 写入的 TraceInfo，未知/异常字段直接跳过。
func decodeTraceRecord(data []byte) traceRecord {
	rec := traceRecord{Extra: map[string]string{}}
	forEachField(data, func(num protowire.Number, val []byte) {
		switch num {
		case 1:
			rec.TraceID = string(val)
		case 2:
			_, rec.APIPath = decodeSubMsg2(val)
		case 4:
			rec.OrgID, rec.UserID = decodeSubMsg2(val)
		case 5:
			rec.AppID, rec.AppType = decodeSubMsg2(val)
		case 6:
			k, v := decodeSubMsg2(val)
			rec.Extra[k] = v
		}
	})
	return rec
}

// mergeTraceRecord 把统计字段并入已有记录：user/org/path 缺失时补全，app 字段直接覆盖。
func mergeTraceRecord(rec *traceRecord, trace DraftRunTrace) {
	if rec.Extra == nil {
		rec.Extra = map[string]string{}
	}
	if rec.UserID == "" {
		rec.UserID = trace.CallerUserID
	}
	if rec.OrgID == "" {
		rec.OrgID = trace.CallerOrgID
	}
	if rec.APIPath == "" {
		rec.APIPath = trace.APIPath
	}
	rec.AppID = trace.WorkflowID
	rec.AppType = trace.AppType
	rec.Extra[TraceExtraSource] = AppStatisticSourceWeb
	rec.Extra[TraceExtraModule] = StatisticModuleWorkflow
	rec.Extra[TraceExtraModuleResourceID] = trace.WorkflowID
	rec.Extra[TraceExtraModuleResourceType] = trace.AppType
	rec.Extra[TraceExtraModuleCreatorUser] = trace.CreatorUserID
	rec.Extra[TraceExtraModuleCreatorOrg] = trace.CreatorOrgID
}
