package wanwu_mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	gomcptransport "github.com/ThinkInAIXYZ/go-mcp/transport"

	"github.com/coze-dev/coze-studio/backend/domain/workflow/entity/vo"
)

type echoArgs struct {
	Text string `json:"text"`
}

func TestBuildMCPTransportCompatibility(t *testing.T) {
	const sseURL = "http://127.0.0.1:9/sse"
	const streamableURL = "http://127.0.0.1:9/mcp"

	tests := []struct {
		name          string
		mcpInfo       string
		wantTransport string
		wantURL       string
		wantError     string
	}{
		{
			name:          "omitted transport uses SSE URL",
			mcpInfo:       fmt.Sprintf(`{"serverUrl":%q,"name":"echo"}`, sseURL),
			wantTransport: MCPTransportSSE,
			wantURL:       sseURL,
		},
		{
			name:          "empty transport uses SSE URL",
			mcpInfo:       fmt.Sprintf(`{"serverUrl":%q,"name":"echo","transport":""}`, sseURL),
			wantTransport: MCPTransportSSE,
			wantURL:       sseURL,
		},
		{
			name:          "explicit SSE uses SSE URL",
			mcpInfo:       fmt.Sprintf(`{"serverUrl":%q,"streamableUrl":%q,"name":"echo","transport":"sse"}`, sseURL, streamableURL),
			wantTransport: MCPTransportSSE,
			wantURL:       sseURL,
		},
		{
			name:          "explicit streamable uses streamable URL",
			mcpInfo:       fmt.Sprintf(`{"serverUrl":%q,"streamableUrl":%q,"name":"echo","transport":"streamable"}`, sseURL, streamableURL),
			wantTransport: MCPTransportStreamable,
			wantURL:       streamableURL,
		},
		{
			name:      "unknown transport is rejected",
			mcpInfo:   fmt.Sprintf(`{"serverUrl":%q,"name":"echo","transport":"stdio"}`, sseURL),
			wantError: "transport not support",
		},
		{
			name:      "legacy node without SSE URL is rejected",
			mcpInfo:   `{"name":"echo"}`,
			wantError: "server url is required",
		},
		{
			name:      "empty transport without SSE URL is rejected",
			mcpInfo:   fmt.Sprintf(`{"streamableUrl":%q,"name":"echo","transport":""}`, streamableURL),
			wantError: "server url is required",
		},
		{
			name:      "explicit SSE without SSE URL is rejected",
			mcpInfo:   fmt.Sprintf(`{"streamableUrl":%q,"name":"echo","transport":"sse"}`, streamableURL),
			wantError: "server url is required",
		},
		{
			name:      "streamable without streamable URL is rejected",
			mcpInfo:   fmt.Sprintf(`{"serverUrl":%q,"name":"echo","transport":"streamable"}`, sseURL),
			wantError: "server url is required",
		},
		{
			name:      "missing tool name is rejected",
			mcpInfo:   fmt.Sprintf(`{"serverUrl":%q}`, sseURL),
			wantError: "mcp tool name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := buildFromMCPInfo(t, tt.mcpInfo)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("Build error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tool.transportType != tt.wantTransport || tool.serverUrl != tt.wantURL {
				t.Fatalf("transport=%q URL=%q, want transport=%q URL=%q", tool.transportType, tool.serverUrl, tt.wantTransport, tt.wantURL)
			}
		})
	}
}

func TestInvokeLegacyAndExplicitMCPTransports(t *testing.T) {
	sseURL := startEchoMCP(t, "sse")
	streamableURL := startEchoMCP(t, "streamable")
	closedSSE := closedURL(t, "/sse")
	closedStreamable := closedURL(t, "/mcp")

	cases := []struct {
		name    string
		mcpInfo string
		args    map[string]any
		want    string
	}{
		{
			name:    "omitted transport",
			mcpInfo: fmt.Sprintf(`{"serverUrl":%q,"name":"echo"}`, sseURL),
			args:    map[string]any{"text": "legacy-omit"},
			want:    "echo:legacy-omit",
		},
		{
			name:    "empty transport",
			mcpInfo: fmt.Sprintf(`{"serverUrl":%q,"name":"echo","transport":""}`, sseURL),
			args:    map[string]any{"text": "legacy-empty"},
			want:    "echo:legacy-empty",
		},
		{
			name:    "explicit SSE ignores streamable URL",
			mcpInfo: fmt.Sprintf(`{"serverUrl":%q,"streamableUrl":%q,"name":"echo","transport":"sse"}`, sseURL, closedStreamable),
			args:    map[string]any{"text": "explicit-sse"},
			want:    "echo:explicit-sse",
		},
		{
			name:    "explicit streamable ignores SSE URL",
			mcpInfo: fmt.Sprintf(`{"serverUrl":%q,"streamableUrl":%q,"name":"echo","transport":"streamable"}`, closedSSE, streamableURL),
			args:    map[string]any{"text": "explicit-streamable"},
			want:    "echo:explicit-streamable",
		},
	}

	var built []*WanWuMCPTool
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := buildFromMCPInfo(t, tt.mcpInfo)
			if err != nil {
				t.Fatal(err)
			}
			out, err := tool.Invoke(context.Background(), tt.args)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), tt.want) {
				t.Fatalf("result %s does not contain %q", encoded, tt.want)
			}
			built = append(built, tool)
		})
	}

	if len(built) != len(cases) {
		t.Fatalf("built %d tools, want %d", len(built), len(cases))
	}
}

func buildFromMCPInfo(t *testing.T, mcpInfo string) (*WanWuMCPTool, error) {
	t.Helper()
	raw := fmt.Sprintf(`{
		"id": "200001",
		"type": "1009",
		"data": {
			"nodeMeta": {"title": "mcp"},
			"inputs": {"mcpInfoList": [%s]},
			"outputs": [{"name": "result", "type": "object"}]
		}
	}`, mcpInfo)
	var node vo.Node
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatalf("unmarshal canvas node: %v", err)
	}
	cfg := &Config{}
	ns, err := cfg.Adapt(context.Background(), &node)
	if err != nil {
		return nil, err
	}
	built, err := cfg.Build(context.Background(), ns)
	if err != nil {
		return nil, err
	}
	tool, ok := built.(*WanWuMCPTool)
	if !ok {
		t.Fatalf("Build returned %T, want *WanWuMCPTool", built)
	}
	return tool, nil
}

func startEchoMCP(t *testing.T, kind string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}

	var transportServer gomcptransport.ServerTransport
	var endpoint string
	switch kind {
	case "sse":
		transportServer, err = gomcptransport.NewSSEServerTransport(addr)
		endpoint = "http://" + addr + "/sse"
	case "streamable":
		transportServer = gomcptransport.NewStreamableHTTPServerTransport(addr)
		endpoint = "http://" + addr + "/mcp"
	default:
		t.Fatalf("unknown echo server kind %s", kind)
	}
	if err != nil {
		t.Fatal(err)
	}

	srv, err := server.NewServer(transportServer, server.WithServerInfo(protocol.Implementation{
		Name:    "echo",
		Version: "0.0.1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	tool, err := protocol.NewTool("echo", "echo the text argument", echoArgs{})
	if err != nil {
		t.Fatal(err)
	}
	srv.RegisterTool(tool, func(_ context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		var args echoArgs
		if len(request.RawArguments) > 0 {
			if err := json.Unmarshal(request.RawArguments, &args); err != nil {
				return nil, err
			}
		}
		text := args.Text
		if text == "" {
			text = "empty"
		}
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "echo:" + text},
			},
		}, nil
	})

	go func() {
		_ = srv.Run()
	}()
	waitTCP(t, addr)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return endpoint
}

func closedURL(t *testing.T, path string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}
	return "http://" + addr + path
}

func waitTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		last = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server %s not ready: %v", addr, last)
}
