package ai

// Role identifies who a Message comes from in a conversation.
type Role string

// The roles a Message can take. Providers map these onto their own wire
// values; RoleTool marks a message that carries tool results back to the
// model.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)
