package main

import (
    "fmt"
    "math/rand"
    "os"
    "os/signal"
    "syscall"
    "time"

    "node_example/client"
    "node_example/protocol"
)

func main() {
    fmt.Println("=== Honeybee Node Example (Go) ===")
    fmt.Println()
    
    // Seed random number generator
    rand.Seed(time.Now().UnixNano())
    
    // Configuration
    nodeID := rand.Uint64()
    nodeName := fmt.Sprintf("go-node-%d", nodeID%1000)
    address := "0.0.0.0"
    port := uint16(8080)
    nodeType := protocol.NodeTypeAgent
    serverAddress := os.Getenv("SERVER_ADDRESS")
    if serverAddress == "" {
        serverAddress = "127.0.0.1:9001"
    }
    
    fmt.Printf("Node ID: %d\n", nodeID)
    fmt.Printf("Node Name: %s\n", nodeName)
    fmt.Printf("Node Type: %s\n", nodeType)
    fmt.Printf("Server: %s\n", serverAddress)
    fmt.Println()
    
    // Create node client
    nodeClient := client.NewNodeClient(nodeID, nodeName, address, port, nodeType, serverAddress)
    
    // Setup signal handling for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    // Run client in goroutine
    errChan := make(chan error, 1)
    go func() {
        errChan <- nodeClient.Run()
    }()
    
    // Wait for shutdown signal or error
    select {
    case <-sigChan:
        fmt.Println("\n[INFO] Received shutdown signal")
        nodeClient.Stop()
    case err := <-errChan:
        if err != nil {
            fmt.Printf("[ERROR] Node client error: %v\n", err)
            os.Exit(1)
        }
    }
    
    fmt.Println("[INFO] Node shutdown complete")
}