package ai

import "context"

type ModelRole string

type modelRoleKey struct{}

const (
	RolePlanner  ModelRole = "planner"
	RoleExecutor ModelRole = "executor"
)

func WithModelRole(ctx context.Context, role ModelRole) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, modelRoleKey{}, role)
}

func modelRoleFromContext(ctx context.Context) ModelRole {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(modelRoleKey{}); value != nil {
		if role, ok := value.(ModelRole); ok {
			return role
		}
	}
	return ""
}

type RoutedClient struct {
	defaultClient Client
	planner       Client
	executor      Client
}

func NewRoutedClient(defaultClient, planner, executor Client) *RoutedClient {
	return &RoutedClient{
		defaultClient: defaultClient,
		planner:       planner,
		executor:      executor,
	}
}

func (c *RoutedClient) Chat(ctx context.Context, messages []Message) (string, error) {
	chosen := c.pick(ctx)
	if chosen == nil {
		return "", ErrNoClient
	}
	return chosen.Chat(ctx, messages)
}

func (c *RoutedClient) ChatWithThought(ctx context.Context, messages []Message) (string, string, error) {
	chosen := c.pick(ctx)
	if chosen == nil {
		return "", "", ErrNoClient
	}
	if client, ok := chosen.(ThoughtClient); ok {
		return client.ChatWithThought(ctx, messages)
	}
	reply, err := chosen.Chat(ctx, messages)
	return reply, "", err
}

func (c *RoutedClient) ChatStream(ctx context.Context, messages []Message, onDelta func(StreamDelta)) (string, string, error) {
	chosen := c.pick(ctx)
	if chosen == nil {
		return "", "", ErrNoClient
	}
	if client, ok := chosen.(StreamClient); ok {
		return client.ChatStream(ctx, messages, onDelta)
	}
	if client, ok := chosen.(ThoughtClient); ok {
		reply, thought, err := client.ChatWithThought(ctx, messages)
		if err == nil && onDelta != nil {
			onDelta(StreamDelta{Content: reply, Thought: thought})
		}
		return reply, thought, err
	}
	reply, err := chosen.Chat(ctx, messages)
	if err == nil && onDelta != nil {
		onDelta(StreamDelta{Content: reply})
	}
	return reply, "", err
}

func (c *RoutedClient) pick(ctx context.Context) Client {
	switch modelRoleFromContext(ctx) {
	case RolePlanner:
		if c.planner != nil {
			return c.planner
		}
	case RoleExecutor:
		if c.executor != nil {
			return c.executor
		}
	}
	if c.defaultClient != nil {
		return c.defaultClient
	}
	if c.executor != nil {
		return c.executor
	}
	return c.planner
}

var ErrNoClient = errNoClient{}

type errNoClient struct{}

func (e errNoClient) Error() string {
	return "ai client is not configured"
}
