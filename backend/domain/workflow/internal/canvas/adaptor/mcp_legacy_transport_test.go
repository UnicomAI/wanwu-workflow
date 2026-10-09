package adaptor

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
	"github.com/coze-dev/coze-studio/backend/domain/workflow/internal/compose"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
)

func TestMixedLegacyAndExplicitSSEMCPWorkflow(t *testing.T) {
	RegisterWanwuAllNodeAdaptors()
	sseURL := startWorkflowEchoMCP(t)

	canvasJSON := fmt.Sprintf(`{
		"nodes": [
			{
				"id": "100001",
				"type": "1",
				"data": {
					"nodeMeta": {"title": "start"},
					"outputs": [{"name": "text", "type": "string", "required": false}]
				}
			},
			{
				"id": "200001",
				"type": "1009",
				"data": {
					"nodeMeta": {"title": "legacy mcp"},
					"inputs": {
						"inputParameters": [{
							"name": "text",
							"input": {"type": "string", "value": {"type": "literal", "content": "legacy-node"}}
						}],
						"mcpInfoList": [{"serverUrl": %q, "name": "echo"}]
					},
					"outputs": [{"name": "result", "type": "object"}]
				}
			},
			{
				"id": "200002",
				"type": "1009",
				"data": {
					"nodeMeta": {"title": "explicit sse mcp"},
					"inputs": {
						"inputParameters": [{
							"name": "text",
							"input": {"type": "string", "value": {"type": "literal", "content": "explicit-node"}}
						}],
						"mcpInfoList": [{"serverUrl": %q, "name": "echo", "transport": "sse", "streamableUrl": ""}]
					},
					"outputs": [{"name": "result", "type": "object"}]
				}
			},
			{
				"id": "900001",
				"type": "2",
				"data": {
					"nodeMeta": {"title": "end"},
					"inputs": {
						"terminatePlan": "returnVariables",
						"inputParameters": [
							{
								"name": "legacy",
								"input": {
									"type": "object",
									"value": {"type": "ref", "content": {"source": "block-output", "blockID": "200001", "name": "result"}}
								}
							},
							{
								"name": "explicit",
								"input": {
									"type": "object",
									"value": {"type": "ref", "content": {"source": "block-output", "blockID": "200002", "name": "result"}}
								}
							}
						]
					}
				}
			}
		],
		"edges": [
			{"sourceNodeID": "100001", "targetNodeID": "200001"},
			{"sourceNodeID": "200001", "targetNodeID": "200002"},
			{"sourceNodeID": "200002", "targetNodeID": "900001"}
		]
	}`, sseURL, sseURL)

	var canvas vo.Canvas
	if err := sonic.UnmarshalString(canvasJSON, &canvas); err != nil {
		t.Fatal(err)
	}
	workflowSC, err := CanvasToWorkflowSchema(context.Background(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := compose.NewWorkflow(context.Background(), workflowSC, compose.WithIDAsName(2))
	if err != nil {
		t.Fatal(err)
	}
	out, err := wf.Runner.Invoke(context.Background(), map[string]any{"text": "unused"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"echo:legacy-node", "echo:explicit-node"} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("workflow output %s does not contain %q", encoded, want)
		}
	}
}

type workflowEchoArgs struct {
	Text string `json:"text"`
}

func startWorkflowEchoMCP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}
	transportServer, err := gomcptransport.NewSSEServerTransport(addr)
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
	tool, err := protocol.NewTool("echo", "echo the text argument", workflowEchoArgs{})
	if err != nil {
		t.Fatal(err)
	}
	srv.RegisterTool(tool, func(_ context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		var args workflowEchoArgs
		if len(request.RawArguments) > 0 {
			if err := json.Unmarshal(request.RawArguments, &args); err != nil {
				return nil, err
			}
		}
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "echo:" + args.Text},
			},
		}, nil
	})
	go func() {
		_ = srv.Run()
	}()
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			last = nil
			break
		}
		last = dialErr
		time.Sleep(50 * time.Millisecond)
	}
	if last != nil {
		t.Fatalf("echo server %s not ready: %v", addr, last)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return "http://" + addr + "/sse"
}
