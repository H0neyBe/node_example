package client

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "sync"
    "time"

    "node_example/protocol"
)

const (
    HeartbeatInterval = 30 * time.Second
    ReconnectDelay    = 5 * time.Second
)

type NodeClient struct {
    nodeID        uint64
    nodeName      string
    address       string
    port          uint16
    nodeType      protocol.NodeType
    serverAddress string
    
    conn      net.Conn
    writer    *bufio.Writer
    reader    *bufio.Reader
    mu        sync.Mutex
    
    stopChan  chan struct{}
    doneChan  chan struct{}
}

func NewNodeClient(nodeID uint64, nodeName, address string, port uint16, nodeType protocol.NodeType, serverAddress string) *NodeClient {
    return &NodeClient{
        nodeID:        nodeID,
        nodeName:      nodeName,
        address:       address,
        port:          port,
        nodeType:      nodeType,
        serverAddress: serverAddress,
        stopChan:      make(chan struct{}),
        doneChan:      make(chan struct{}),
    }
}

func (nc *NodeClient) connect() error {
    fmt.Printf("[INFO] Connecting to server at %s...\n", nc.serverAddress)
    
    conn, err := net.Dial("tcp", nc.serverAddress)
    if err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }
    
    nc.conn = conn
    nc.writer = bufio.NewWriter(conn)
    nc.reader = bufio.NewReader(conn)
    
    fmt.Println("[INFO] Connected to server successfully")
    return nil
}

func (nc *NodeClient) sendMessage(envelope protocol.MessageEnvelope) error {
    nc.mu.Lock()
    defer nc.mu.Unlock()
    
    data, err := json.Marshal(envelope)
    if err != nil {
        return fmt.Errorf("failed to marshal message: %w", err)
    }
    
    // Write message + newline, then flush immediately
    if _, err := nc.writer.Write(data); err != nil {
        return fmt.Errorf("failed to write message: %w", err)
    }
    
    if err := nc.writer.WriteByte('\n'); err != nil {
        return fmt.Errorf("failed to write newline: %w", err)
    }
    
    if err := nc.writer.Flush(); err != nil {
        return fmt.Errorf("failed to flush: %w", err)
    }
    
    return nil
}

func (nc *NodeClient) register() error {
    fmt.Println("[INFO] Sending registration...")
    
    registration := protocol.NodeRegistration{
        NodeID:   nc.nodeID,
        NodeName: nc.nodeName,
        Address:  nc.address,
        Port:     nc.port,
        NodeType: nc.nodeType,
    }
    
    envelope := protocol.MessageEnvelope{
        Version: protocol.ProtocolVersion,
        Message: protocol.MessageType{
            NodeRegistration: &registration,
        },
    }
    
    if err := nc.sendMessage(envelope); err != nil {
        return err
    }
    
    fmt.Println("[INFO] Registration sent successfully")
    return nil
}

func (nc *NodeClient) sendStatusUpdate(status protocol.NodeStatus) error {
    fmt.Printf("[INFO] Sending status update: %s\n", status)
    
    statusUpdate := protocol.NodeStatusUpdate{
        NodeID: nc.nodeID,
        Status: status,
    }
    
    envelope := protocol.MessageEnvelope{
        Version: protocol.ProtocolVersion,
        Message: protocol.MessageType{
            NodeStatusUpdate: &statusUpdate,
        },
    }
    
    return nc.sendMessage(envelope)
}

func (nc *NodeClient) handleMessage(envelope protocol.MessageEnvelope) error {
    if envelope.Message.NodeCommand != nil {
        cmd := envelope.Message.NodeCommand
        fmt.Printf("[INFO] Received command: %s\n", cmd.Command)
        
        switch cmd.Command {
        case "stop":
            if err := nc.sendStatusUpdate(protocol.NodeStatusStopped); err != nil {
                return err
            }
            fmt.Println("[INFO] Node stopped by command")
            close(nc.stopChan)
        case "status":
            if err := nc.sendStatusUpdate(protocol.NodeStatusRunning); err != nil {
                return err
            }
        default:
            fmt.Printf("[WARN] Unknown command: %s\n", cmd.Command)
        }
    }
    return nil
}

func (nc *NodeClient) readMessages(errorChan chan<- error) {
    for {
        select {
        case <-nc.stopChan:
            return
        default:
            line, err := nc.reader.ReadString('\n')
            if err != nil {
                errorChan <- fmt.Errorf("read error: %w", err)
                return
            }
            
            var envelope protocol.MessageEnvelope
            if err := json.Unmarshal([]byte(line), &envelope); err != nil {
                fmt.Printf("[WARN] Failed to parse message: %v\n", err)
                continue
            }
            
            if err := nc.handleMessage(envelope); err != nil {
                fmt.Printf("[ERROR] Failed to handle message: %v\n", err)
            }
        }
    }
}

func (nc *NodeClient) sendHeartbeats(errorChan chan<- error) {
    ticker := time.NewTicker(HeartbeatInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-nc.stopChan:
            return
        case <-ticker.C:
            if err := nc.sendStatusUpdate(protocol.NodeStatusRunning); err != nil {
                errorChan <- fmt.Errorf("heartbeat failed: %w", err)
                return
            }
            fmt.Println("[INFO] Heartbeat sent")
        }
    }
}

func (nc *NodeClient) Run() error {
    defer close(nc.doneChan)
    
    for {
        select {
        case <-nc.stopChan:
            fmt.Println("[INFO] Shutting down...")
            return nil
        default:
        }
        
        // Connect to server
        if err := nc.connect(); err != nil {
            fmt.Printf("[ERROR] Connection failed: %v\n", err)
            time.Sleep(ReconnectDelay)
            continue
        }
        
        // Register with server
        if err := nc.register(); err != nil {
            fmt.Printf("[ERROR] Registration failed: %v\n", err)
            nc.conn.Close()
            time.Sleep(ReconnectDelay)
            continue
        }
        
        // Small delay to ensure registration is processed before status update
        time.Sleep(100 * time.Millisecond)
        
        // Send initial status
        if err := nc.sendStatusUpdate(protocol.NodeStatusRunning); err != nil {
            fmt.Printf("[ERROR] Failed to send initial status: %v\n", err)
            nc.conn.Close()
            time.Sleep(ReconnectDelay)
            continue
        }
        
        // Start background tasks
        errorChan := make(chan error, 2)
        
        go nc.readMessages(errorChan)
        go nc.sendHeartbeats(errorChan)
        
        // Wait for error or stop signal
        select {
        case err := <-errorChan:
            fmt.Printf("[ERROR] %v\n", err)
            nc.conn.Close()
            fmt.Printf("[INFO] Reconnecting in %v...\n", ReconnectDelay)
            time.Sleep(ReconnectDelay)
        case <-nc.stopChan:
            nc.conn.Close()
            return nil
        }
    }
}

func (nc *NodeClient) Stop() {
    close(nc.stopChan)
    <-nc.doneChan
}