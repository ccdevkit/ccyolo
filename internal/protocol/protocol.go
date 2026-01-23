package protocol

// RequestType identifies the type of request in the TCP protocol
type RequestType string

const (
	TypeExec RequestType = "exec"
	TypeLog  RequestType = "log"
)

// ExecRequest is sent from container to host for command execution
type ExecRequest struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

// LogRequest is sent from container to host for debug logging
type LogRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
