package wanwu_mcp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	net_url "net/url"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/canvas/convert"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes"
	wanwu_util "github.com/coze-dev/coze-studio/backend/domain/workflow/internal/nodes/wanwu-util"
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/schema"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

const (
	WanWuMCPResult         = "result"
	MCPTransportSSE        = "sse"
	MCPTransportStreamable = "streamable"
)

type Config struct {
	SseUrl        string
	McpToolName   string
	Transport     string                       // 传输协议: "sse" 或 "streamable"
	StreamableUrl string                       // Streamable HTTP URL
	ApiAuth       wanwu_util.ApiAuthWebRequest // 鉴权信息
	Headers       map[string]string            // 请求头
}

func (c *Config) Adapt(_ context.Context, n *vo.Node, _ ...nodes.AdaptOption) (*schema.NodeSchema, error) {
	inputs := n.Data.Inputs
	mcpToolInfoList := inputs.McpToolInfoList

	if len(mcpToolInfoList) > 0 {
		mcpInfo := mcpToolInfoList[0]
		c.SseUrl = mcpInfo.MCPServerURL
		c.McpToolName = mcpInfo.ToolName
		c.Transport = mcpInfo.Transport
		c.StreamableUrl = mcpInfo.StreamableURL
		c.ApiAuth = mcpInfo.ApiAuth
		c.Headers = mcpInfo.Headers
	} else {
		return nil, errors.New("未找到MCP工具配置信息")
	}

	ns := &schema.NodeSchema{
		Key:     vo.NodeKey(n.ID),
		Type:    entity.NodeTypeWanWuMCPTool,
		Name:    n.Data.Meta.Title,
		Configs: c,
	}

	if err := convert.SetInputsForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	if err := convert.SetOutputTypesForNodeSchema(n, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

func (c *Config) Build(_ context.Context, ns *schema.NodeSchema, _ ...schema.BuildOption) (any, error) {
	// 根据 transport 类型选择 URL
	var serverUrl string
	var transportType string
	switch c.Transport {
	case MCPTransportStreamable:
		serverUrl = c.StreamableUrl
		transportType = MCPTransportStreamable
	case MCPTransportSSE:
		serverUrl = c.SseUrl
		transportType = MCPTransportSSE
	default:
		return nil, errors.New("transport not support")
	}

	if serverUrl == "" {
		return nil, errors.New("server url is required")
	}

	if c.McpToolName == "" {
		return nil, errors.New("mcp tool name is required")
	}

	tool := &WanWuMCPTool{
		serverUrl:     serverUrl,
		mcpToolName:   c.McpToolName,
		transportType: transportType,
		apiAuth:       c.ApiAuth,
		headers:       c.Headers,
	}
	return tool, nil
}

type WanWuMCPTool struct {
	serverUrl     string
	mcpToolName   string
	transportType string
	apiAuth       wanwu_util.ApiAuthWebRequest
	headers       map[string]string
	mcpToolArgs   map[string]any
}

// httpClient 创建共享的 HTTP 客户端，跳过证书验证
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

// headerTransport 是一个 http.RoundTripper 包装器，用于注入自定义请求头
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.base.RoundTrip(req)
}

// newHTTPClientWithHeaders 创建带有header息的 HTTP 客户端
func newHTTPClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return httpClient
	}

	return &http.Client{
		Transport: &headerTransport{
			base: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			headers: headers,
		},
	}
}

func (i *WanWuMCPTool) Invoke(ctx context.Context, in map[string]any) (map[string]any, error) {
	i.mcpToolArgs = in

	var transportClient transport.ClientTransport
	var err error

	mergedUrl, mergedHeaders, err := MergeMcpParams(i.serverUrl, i.apiAuth, i.headers)
	if err != nil {
		return nil, err
	}

	// 构建带鉴权的 HTTP 客户端
	clientWithAuth := newHTTPClientWithHeaders(mergedHeaders)

	switch i.transportType {
	case MCPTransportStreamable:
		// 创建 StreamableHTTP 传输客户端
		transportClient, err = transport.NewStreamableHTTPClientTransport(mergedUrl,
			transport.WithStreamableHTTPClientOptionLogger(logs.DefaultLogger()),
			transport.WithStreamableHTTPClientOptionHTTPClient(clientWithAuth),
		)
		if err != nil {
			return nil, err
		}
	case MCPTransportSSE:
		// 默认使用 SSE 传输客户端
		transportClient, err = transport.NewSSEClientTransport(mergedUrl,
			transport.WithSSEClientOptionReceiveTimeout(time.Minute*2),
			transport.WithSSEClientOptionLogger(logs.DefaultLogger()),
			transport.WithSSEClientOptionHTTPClient(clientWithAuth),
		)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("transport not support")
	}

	mcpClient, err := client.NewClient(transportClient)
	if err != nil {
		return nil, err
	}
	defer mcpClient.Close()

	request := &protocol.CallToolRequest{
		Name:      i.mcpToolName,
		Arguments: i.mcpToolArgs,
	}

	toolCallResult, err := mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, err
	}

	resultBytes, err := json.Marshal(toolCallResult)
	if err != nil {
		return nil, err
	}

	var resultMap map[string]any
	if err := json.Unmarshal(resultBytes, &resultMap); err != nil {
		return nil, err
	}

	return map[string]any{
		"result": resultMap,
	}, nil
}

// MergeMcpParams 将鉴权中的header参数合并到headers，鉴权的query参数追加url
func MergeMcpParams(url string, auth wanwu_util.ApiAuthWebRequest, headers map[string]string) (mergedUrl string, mergedHeaders map[string]string, err error) {
	if (auth.AuthType == "" || auth.AuthType == util.AuthTypeNone) && len(headers) == 0 {
		return url, headers, nil
	}
	mergedHeaders = make(map[string]string)
	mergedUrl = url

	for k, v := range headers {
		mergedHeaders[k] = v
	}

	if auth.AuthType != "" && auth.AuthType != util.AuthTypeNone {
		switch auth.AuthType {
		case util.AuthTypeAPIKeyHeader:
			value := auth.ApiKeyValue
			switch auth.ApiKeyHeaderPrefix {
			case util.ApiKeyHeaderPrefixBasic:
				value = "Basic " + auth.ApiKeyValue
			case util.ApiKeyHeaderPrefixBearer:
				value = "Bearer " + auth.ApiKeyValue
			}
			headerName := auth.ApiKeyHeader
			if headerName == "" {
				headerName = util.ApiKeyHeaderDefault
			}
			if _, ok := mergedHeaders[headerName]; ok {
				return url, mergedHeaders, fmt.Errorf("header %s already exists", headerName)
			}
			mergedHeaders[headerName] = value
		case util.AuthTypeAPIKeyQuery:
			rawUrl, err := net_url.Parse(url)
			if err != nil {
				return url, mergedHeaders, err
			}
			queryParams := rawUrl.Query()
			if queryParams.Has(auth.ApiKeyQueryParam) {
				return url, mergedHeaders, fmt.Errorf("query param %s already exists", auth.ApiKeyQueryParam)
			}
			queryParams.Add(auth.ApiKeyQueryParam, auth.ApiKeyValue)
			rawUrl.RawQuery = queryParams.Encode()
			mergedUrl = rawUrl.String()
		}
	}
	return mergedUrl, mergedHeaders, nil
}
