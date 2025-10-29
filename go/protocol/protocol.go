package protocol

type NodeType string

const (
    NodeTypeFull  NodeType = "Full"
    NodeTypeAgent NodeType = "Agent"
)

type NodeStatus string

const (
    NodeStatusDeploying NodeStatus = "Deploying"
    NodeStatusRunning   NodeStatus = "Running"
    NodeStatusStopped   NodeStatus = "Stopped"
    NodeStatusFailed    NodeStatus = "Failed"
    NodeStatusUnknown   NodeStatus = "Unknown"
)

type MessageEnvelope struct {
    Version uint64      `json:"version"`
    Message MessageType `json:"message"`
}

type MessageType struct {
    NodeRegistration *NodeRegistration `json:"NodeRegistration,omitempty"`
    NodeStatusUpdate *NodeStatusUpdate `json:"NodeStatusUpdate,omitempty"`
    NodeCommand      *NodeCommand      `json:"NodeCommand,omitempty"`
}

type NodeRegistration struct {
    NodeID   uint64   `json:"node_id"`
    NodeName string   `json:"node_name"`
    Address  string   `json:"address"`
    Port     uint16   `json:"port"`
    NodeType NodeType `json:"node_type"`
}

type NodeStatusUpdate struct {
    NodeID uint64     `json:"node_id"`
    Status NodeStatus `json:"status"`
}

type NodeCommand struct {
    NodeID  uint64 `json:"node_id"`
    Command string `json:"command"`
}

const ProtocolVersion uint64 = 1