use bee_message::node::{MessageEnvelope, MessageType, NodeCommand, NodeRegistration, NodeStatus, NodeStatusUpdate, NodeType};
use std::error::Error;
use std::time::Duration;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::time;

const PROTOCOL_VERSION: u64 = 1;
const HEARTBEAT_INTERVAL: Duration = Duration::from_secs(30);

struct NodeClient {
  node_id:        u64,
  node_name:      String,
  address:        String,
  port:           u16,
  node_type:      NodeType,
  server_address: String,
}

impl NodeClient {
  fn new(node_id: u64, node_name: String, address: String, port: u16, node_type: NodeType, server_address: String) -> Self {
    NodeClient {
      node_id,
      node_name,
      address,
      port,
      node_type,
      server_address,
    }
  }

  async fn connect(&self) -> Result<TcpStream, Box<dyn Error>> {
    println!("Connecting to server at {}...", self.server_address);
    let stream = TcpStream::connect(&self.server_address).await?;
    println!("Connected to server!");
    Ok(stream)
  }

  async fn register(&self, stream: &mut TcpStream) -> Result<(), Box<dyn Error>> {
    println!("Sending registration...");

    let registration = NodeRegistration {
      node_id:   self.node_id,
      node_name: self.node_name.clone(),
      address:   self.address.clone(),
      port:      self.port,
      node_type: self.node_type.clone(),
    };

    let envelope = MessageEnvelope {
      version: PROTOCOL_VERSION,
      message: MessageType::NodeRegistration(registration),
    };

    let json = serde_json::to_string(&envelope)?;
    stream.write_all(json.as_bytes()).await?;
    stream.flush().await?;

    println!("Registration sent successfully!");
    Ok(())
  }

  async fn send_status_update(&self, stream: &mut TcpStream, status: NodeStatus) -> Result<(), Box<dyn Error>> {
    println!("Sending status update: {:?}", status);

    let status_update = NodeStatusUpdate {
      node_id: self.node_id,
      status,
    };

    let envelope = MessageEnvelope {
      version: PROTOCOL_VERSION,
      message: MessageType::NodeStatusUpdate(status_update),
    };

    let json = serde_json::to_string(&envelope)?;
    stream.write_all(json.as_bytes()).await?;
    stream.flush().await?;

    Ok(())
  }

  async fn run(&self) -> Result<(), Box<dyn Error>> {
    loop {
      match self.connect().await {
        Ok(mut stream) => {
          // Register with the server
          if let Err(e) = self.register(&mut stream).await {
            eprintln!("Failed to register: {}", e);
            time::sleep(Duration::from_secs(5)).await;
            continue;
          }
          time::sleep(Duration::from_secs(1)).await;

          // Send initial status
          if let Err(e) = self.send_status_update(&mut stream, NodeStatus::Running).await {
            eprintln!("Failed to send initial status: {}", e);
            time::sleep(Duration::from_secs(5)).await;
            continue;
          }

          // Maintain connection with heartbeats
          let mut interval = time::interval(HEARTBEAT_INTERVAL);
          let mut buf = vec![0u8; 4096];

          loop {
            tokio::select! {
                // Send periodic heartbeat
                _ = interval.tick() => {
                    if let Err(e) = self.send_status_update(&mut stream, NodeStatus::Running).await {
                        eprintln!("Heartbeat failed: {}", e);
                        break;
                    }
                    println!("Heartbeat sent");
                }
                // Read incoming messages
                result = stream.read(&mut buf) => {
                    match result {
                        Ok(0) => {
                            println!("Connection closed by server");
                            break;
                        }
                        Ok(n) => {
                            let msg = String::from_utf8_lossy(&buf[..n]);
                            println!("Received message: {}", msg);

                            // Try to parse as MessageEnvelope
                            if let Ok(envelope) = serde_json::from_str::<MessageEnvelope>(&msg) {
                                self.handle_message(envelope, &mut stream).await?;
                            }
                        }
                        Err(e) => {
                            eprintln!("Read error: {}", e);
                            break;
                        }
                    }
                }
            }
          }
        }
        Err(e) => {
          eprintln!("Connection failed: {}", e);
        }
      }

      println!("Reconnecting in 5 seconds...");
      time::sleep(Duration::from_secs(5)).await;
    }
  }

  async fn handle_message(&self, envelope: MessageEnvelope, stream: &mut TcpStream) -> Result<(), Box<dyn Error>> {
    match envelope.message {
      MessageType::NodeCommand(cmd) => {
        println!("Received command: {}", cmd.command);

        // Execute command and send status update
        match cmd.command.as_str() {
          "stop" => {
            self.send_status_update(stream, NodeStatus::Stopped).await?;
            println!("Node stopped by command");
          }
          "status" => {
            self.send_status_update(stream, NodeStatus::Running).await?;
          }
          _ => {
            println!("Unknown command: {}", cmd.command);
          }
        }
      }
      _ => {
        println!("Received unexpected message type");
      }
    }
    Ok(())
  }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
  println!("=== Honeybee Node Example (Rust) ===");

  // Configuration
  let node_id = rand::random::<u64>();
  let node_name = format!("rust-node-{}", node_id % 1000);
  let address = "0.0.0.0".to_string();
  let port = 8080;
  let node_type = NodeType::Agent;
  let server_address = std::env::var("SERVER_ADDRESS").unwrap_or_else(|_| "127.0.0.1:9001".to_string());

  println!("Node ID: {}", node_id);
  println!("Node Name: {}", node_name);
  println!("Node Type: {:?}", node_type);
  println!("Server: {}", server_address);
  println!();

  let client = NodeClient::new(node_id, node_name, address, port, node_type, server_address);

  client.run().await
}
