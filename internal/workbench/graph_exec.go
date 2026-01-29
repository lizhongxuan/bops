package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bops/internal/ai"
	"bops/internal/logging"
	"bops/internal/runmanager"
	"bops/internal/scheduler"
	"bops/internal/workflow"
	"go.uber.org/zap"
)

type GraphContext struct {
	Inputs  map[string]any
	Outputs map[string]map[string]any
	System  map[string]any
}

func NewGraphContext(inputs map[string]any) *GraphContext {
	out := map[string]any{}
	for k, v := range inputs {
		out[k] = v
	}
	return &GraphContext{
		Inputs:  out,
		Outputs: map[string]map[string]any{},
		System:  map[string]any{},
	}
}

func (c *GraphContext) SetOutput(id string, output map[string]any) {
	if output == nil {
		output = map[string]any{}
	}
	c.Outputs[id] = output
}

func (c *GraphContext) Lookup(selector string) (any, bool) {
	if selector == "" {
		return nil, false
	}
	parts := strings.Split(selector, ".")
	if len(parts) == 0 {
		return nil, false
	}
	head := parts[0]
	rest := parts[1:]
	if head == "sys" {
		return lookupPath(c.System, rest)
	}
	if head == "inputs" {
		return lookupPath(c.Inputs, rest)
	}
	if output, ok := c.Outputs[head]; ok {
		return lookupPath(output, rest)
	}
	return nil, false
}

func lookupPath(value any, path []string) (any, bool) {
	if len(path) == 0 {
		return value, true
	}
	switch v := value.(type) {
	case map[string]any:
		next, ok := v[path[0]]
		if !ok {
			return nil, false
		}
		return lookupPath(next, path[1:])
	case map[any]any:
		next, ok := v[path[0]]
		if !ok {
			return nil, false
		}
		return lookupPath(next, path[1:])
	case []any:
		idx, err := strconv.Atoi(path[0])
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, false
		}
		return lookupPath(v[idx], path[1:])
	default:
		return nil, false
	}
}

var tokenPattern = regexp.MustCompile(`\{\{#([^#]+)#\}\}`)

func resolveValue(value any, ctx *GraphContext) any {
	switch v := value.(type) {
	case string:
		return resolveString(v, ctx)
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			result = append(result, resolveValue(item, ctx))
		}
		return result
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = resolveValue(item, ctx)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[fmt.Sprint(key)] = resolveValue(item, ctx)
		}
		return out
	default:
		return value
	}
}

func resolveString(input string, ctx *GraphContext) any {
	matches := tokenPattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input
	}
	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(input) {
		selector := input[matches[0][2]:matches[0][3]]
		if value, ok := ctx.Lookup(selector); ok {
			return value
		}
		return ""
	}
	var builder strings.Builder
	offset := 0
	for _, match := range matches {
		builder.WriteString(input[offset:match[0]])
		selector := input[match[2]:match[3]]
		if value, ok := ctx.Lookup(selector); ok {
			builder.WriteString(fmt.Sprint(value))
		}
		offset = match[1]
	}
	builder.WriteString(input[offset:])
	return builder.String()
}

type GraphExecutor struct {
	AI   ai.Client
	HTTP *http.Client
}

func (e *GraphExecutor) Run(ctx context.Context, graph Graph, inputs map[string]any, recorder *runmanager.Recorder) (map[string]any, error) {
	if strings.TrimSpace(graph.Version) == "" {
		graph.Version = "v1"
	}
	order := topoOrder(graph)
	nodeMap := make(map[string]Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeMap[node.ID] = node
	}

	execCtx := NewGraphContext(inputs)
	execCtx.System["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	if len(order) == 0 {
		return nil, fmt.Errorf("graph is empty")
	}

	var lastOutput map[string]any
	for _, id := range order {
		node, ok := nodeMap[id]
		if !ok {
			continue
		}
		stepName := strings.TrimSpace(node.Name)
		if stepName == "" {
			stepName = node.ID
		}
		step := workflow.Step{
			Name:   stepName,
			Action: node.Type,
		}
		if recorder != nil {
			recorder.StepStart(step, nil)
		}
		resolved := resolveValue(node.Data, execCtx)
		data, ok := resolved.(map[string]any)
		if !ok {
			data = map[string]any{}
		}

		output, err := e.executeNode(ctx, node, data, execCtx)
		if err != nil {
			if recorder != nil {
				recorder.StepFinish(step, "failed")
				recorder.HostResult(step, workflow.HostSpec{Name: node.Type}, scheduler.Result{
					Status: "failed",
					Error:  err.Error(),
				})
			}
			return nil, err
		}
		execCtx.SetOutput(node.ID, output)
		if node.Type == "start" {
			execCtx.SetOutput("start", output)
		}
		if node.Type == "end" {
			execCtx.SetOutput("end", output)
		}
		lastOutput = output
		if recorder != nil {
			recorder.StepFinish(step, "success")
			recorder.HostResult(step, workflow.HostSpec{Name: node.Type}, scheduler.Result{
				Status: "success",
				Output: output,
			})
		}
	}

	if out, ok := execCtx.Outputs["end"]; ok {
		return out, nil
	}
	return lastOutput, nil
}

func (e *GraphExecutor) executeNode(ctx context.Context, node Node, data map[string]any, execCtx *GraphContext) (map[string]any, error) {
	switch strings.TrimSpace(node.Type) {
	case "start":
		return execStartNode(data, execCtx.Inputs), nil
	case "llm":
		return execLLMNode(ctx, data, e.AI)
	case "knowledge-retrieval":
		return execKnowledgeNode(data), nil
	case "code":
		return execCodeNode(data), nil
	case "http-request":
		return execHTTPNode(ctx, data, e.HTTP)
	case "if-else":
		return execIfElseNode(data), nil
	case "iteration":
		return execIterationNode(data), nil
	case "tool":
		return execToolNode(data)
	case "variable-assigner":
		return execVariableAssignerNode(data), nil
	case "end":
		return execEndNode(data), nil
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

func execStartNode(data map[string]any, inputs map[string]any) map[string]any {
	out := map[string]any{}
	raw := data["inputs"]
	items, ok := raw.([]any)
	if !ok {
		for key, value := range inputs {
			out[key] = value
		}
		return out
	}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(entry["name"]))
		if name == "" {
			continue
		}
		if value, ok := inputs[name]; ok {
			out[name] = value
			continue
		}
		if def, ok := entry["default"]; ok {
			out[name] = def
			continue
		}
		out[name] = ""
	}
	return out
}

func execLLMNode(ctx context.Context, data map[string]any, client ai.Client) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("ai client not configured")
	}
	prompt := strings.TrimSpace(fmt.Sprint(data["prompt"]))
	if prompt == "" {
		return nil, fmt.Errorf("llm.prompt is required")
	}
	messages := []ai.Message{}
	if system, ok := data["system"]; ok {
		sysText := strings.TrimSpace(fmt.Sprint(system))
		if sysText != "" {
			messages = append(messages, ai.Message{Role: "system", Content: sysText})
		}
	}
	messages = append(messages, ai.Message{Role: "user", Content: prompt})
	reply, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"text": reply,
	}, nil
}

func execKnowledgeNode(data map[string]any) map[string]any {
	return map[string]any{
		"documents": []any{},
		"query":     data["query"],
	}
}

func execCodeNode(data map[string]any) map[string]any {
	inputs := map[string]any{}
	if raw, ok := data["inputs"]; ok {
		if items, ok := raw.([]any); ok {
			for _, item := range items {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				key := strings.TrimSpace(fmt.Sprint(entry["name"]))
				if key == "" {
					continue
				}
				inputs[key] = entry["value"]
			}
		}
	}
	outputs := map[string]any{}
	if raw, ok := data["outputs"]; ok {
		if items, ok := raw.([]any); ok {
			for _, item := range items {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				key := strings.TrimSpace(fmt.Sprint(entry["name"]))
				if key == "" {
					continue
				}
				outputs[key] = entry["value"]
			}
		}
	}
	if len(outputs) == 0 {
		outputs["result"] = inputs
	}
	outputs["inputs"] = inputs
	return outputs
}

func execHTTPNode(ctx context.Context, data map[string]any, client *http.Client) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(fmt.Sprint(data["method"])))
	if method == "" {
		method = http.MethodGet
	}
	url := strings.TrimSpace(fmt.Sprint(data["url"]))
	if url == "" {
		return nil, fmt.Errorf("http-request.url is required")
	}
	var bodyReader *strings.Reader
	if body, ok := data["body"]; ok {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			payload, _ := json.Marshal(v)
			bodyReader = strings.NewReader(string(payload))
		}
	} else {
		bodyReader = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	headers, _ := data["headers"].(map[string]any)
	for key, value := range headers {
		req.Header.Set(key, fmt.Sprint(value))
	}
	if req.Header.Get("Content-Type") == "" && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return map[string]any{
		"status":  resp.StatusCode,
		"body":    string(bodyBytes),
		"headers": resp.Header,
	}, nil
}

func execIfElseNode(data map[string]any) map[string]any {
	result := false
	raw := data["conditions"]
	if items, ok := raw.([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if evaluateCondition(entry) {
				result = true
				break
			}
		}
	}
	return map[string]any{
		"result": result,
	}
}

func evaluateCondition(cond map[string]any) bool {
	left := fmt.Sprint(cond["left"])
	right := fmt.Sprint(cond["right"])
	op := strings.ToLower(strings.TrimSpace(fmt.Sprint(cond["operator"])))
	switch op {
	case "contains":
		return strings.Contains(left, right)
	case "equals", "==":
		return left == right
	case "!=", "not_equals":
		return left != right
	case ">", ">=", "<", "<=":
		ln, lerr := strconv.ParseFloat(left, 64)
		rn, rerr := strconv.ParseFloat(right, 64)
		if lerr != nil || rerr != nil {
			return false
		}
		switch op {
		case ">":
			return ln > rn
		case ">=":
			return ln >= rn
		case "<":
			return ln < rn
		case "<=":
			return ln <= rn
		}
	}
	return false
}

func execIterationNode(data map[string]any) map[string]any {
	raw := data["array"]
	if items, ok := raw.([]any); ok {
		return map[string]any{
			"items": items,
			"count": len(items),
		}
	}
	return map[string]any{
		"items": []any{},
		"count": 0,
	}
}

func execToolNode(data map[string]any) (map[string]any, error) {
	name := strings.ToLower(strings.TrimSpace(fmt.Sprint(data["name"])))
	switch name {
	case "calculator":
		params, _ := data["params"].(map[string]any)
		expr := strings.TrimSpace(fmt.Sprint(params["expression"]))
		if expr == "" {
			return nil, fmt.Errorf("calculator.expression is required")
		}
		value, err := evalSimpleExpression(expr)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": value}, nil
	default:
		return nil, fmt.Errorf("unsupported tool: %s", name)
	}
}

func execVariableAssignerNode(data map[string]any) map[string]any {
	output := map[string]any{}
	raw := data["assignments"]
	if items, ok := raw.([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key := strings.TrimSpace(fmt.Sprint(entry["name"]))
			if key == "" {
				continue
			}
			output[key] = entry["value"]
		}
	}
	return output
}

func execEndNode(data map[string]any) map[string]any {
	output := map[string]any{}
	raw := data["outputs"]
	if items, ok := raw.([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key := strings.TrimSpace(fmt.Sprint(entry["name"]))
			if key == "" {
				continue
			}
			output[key] = entry["value"]
		}
	}
	if len(output) == 0 {
		output["result"] = data
	}
	return output
}

func evalSimpleExpression(expr string) (float64, error) {
	parser := newExpressionParser(expr)
	value, err := parser.parse()
	if err != nil {
		return 0, err
	}
	if !parser.isEOF() {
		return 0, fmt.Errorf("invalid expression")
	}
	return value, nil
}

type expressionParser struct {
	input string
	pos   int
}

func newExpressionParser(input string) *expressionParser {
	return &expressionParser{input: strings.ReplaceAll(input, " ", ""), pos: 0}
}

func (p *expressionParser) parse() (float64, error) {
	return p.parseExpr()
}

func (p *expressionParser) parseExpr() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		if p.match('+') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += next
			continue
		}
		if p.match('-') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= next
			continue
		}
		break
	}
	return value, nil
}

func (p *expressionParser) parseTerm() (float64, error) {
	value, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		if p.match('*') {
			next, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			value *= next
			continue
		}
		if p.match('/') {
			next, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			value /= next
			continue
		}
		break
	}
	return value, nil
}

func (p *expressionParser) parseFactor() (float64, error) {
	if p.match('(') {
		value, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if !p.match(')') {
			return 0, fmt.Errorf("missing )")
		}
		return value, nil
	}
	if p.match('-') {
		value, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -value, nil
	}
	return p.parseNumber()
}

func (p *expressionParser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number")
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}

func (p *expressionParser) match(ch byte) bool {
	if p.pos >= len(p.input) || p.input[p.pos] != ch {
		return false
	}
	p.pos++
	return true
}

func (p *expressionParser) isEOF() bool {
	return p.pos >= len(p.input)
}

func (e *GraphExecutor) LogGraphRun(graph Graph) {
	logging.L().Debug("graph run",
		zap.String("version", graph.Version),
		zap.Int("nodes", len(graph.Nodes)),
		zap.Int("edges", len(graph.Edges)),
	)
}
