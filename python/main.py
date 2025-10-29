import asyncio
import json
import random
import os
from enum import Enum
from typing import Optional, Dict, Any
from dataclasses import dataclass, asdict

PROTOCOL_VERSION = 1
HEARTBEAT_INTERVAL = 30
RECONNECT_DELAY = 5


class NodeType(str, Enum):
    FULL = "Full"
    AGENT = "Agent"


class NodeStatus(str, Enum):
    DEPLOYING = "Deploying"
    RUNNING = "Running"
    STOPPED = "Stopped"
    FAILED = "Failed"
    UNKNOWN = "Unknown"


@dataclass
class NodeRegistration:
    node_id: int
    node_name: str
    address: str
    port: int
    node_type: str


@dataclass
class NodeStatusUpdate:
    node_id: int
    status: str


@dataclass
class NodeCommand:
    node_id: int
    command: str


@dataclass
class MessageEnvelope:
    version: int
    message: Dict[str, Any]


class NodeClient:
    def __init__(
        self,
        node_id: int,
        node_name: str,
        address: str,
        port: int,
        node_type: NodeType,
        server_address: str,
    ):
        self.node_id = node_id
        self.node_name = node_name
        self.address = address
        self.port = port
        self.node_type = node_type
        self.server_host, self.server_port = server_address.split(":")
        self.server_port = int(self.server_port)

    async def connect(self) -> tuple:
        print(f"Connecting to server at {self.server_host}:{self.server_port}...")
        reader, writer = await asyncio.open_connection(
            self.server_host, self.server_port
        )
        print("Connected to server!")
        return reader, writer

    async def register(self, writer: asyncio.StreamWriter) -> None:
        print("Sending registration...")

        registration = NodeRegistration(
            node_id=self.node_id,
            node_name=self.node_name,
            address=self.address,
            port=self.port,
            node_type=self.node_type.value,
        )

        envelope = MessageEnvelope(
            version=PROTOCOL_VERSION,
            message={"NodeRegistration": asdict(registration)},
        )

        message = json.dumps(asdict(envelope))
        writer.write(message.encode())
        await writer.drain()

        print("Registration sent successfully!")

    async def send_status_update(
        self, writer: asyncio.StreamWriter, status: NodeStatus
    ) -> None:
        print(f"Sending status update: {status.value}")

        status_update = NodeStatusUpdate(node_id=self.node_id, status=status.value)

        envelope = MessageEnvelope(
            version=PROTOCOL_VERSION,
            message={"NodeStatusUpdate": asdict(status_update)},
        )

        message = json.dumps(asdict(envelope))
        writer.write(message.encode())
        await writer.drain()

    async def handle_message(
        self, envelope: Dict[str, Any], writer: asyncio.StreamWriter
    ) -> None:
        message = envelope.get("message", {})

        if "NodeCommand" in message:
            cmd = message["NodeCommand"]
            print(f"Received command: {cmd['command']}")

            if cmd["command"] == "stop":
                await self.send_status_update(writer, NodeStatus.STOPPED)
                print("Node stopped by command")
            elif cmd["command"] == "status":
                await self.send_status_update(writer, NodeStatus.RUNNING)
            else:
                print(f"Unknown command: {cmd['command']}")
        else:
            print("Received unexpected message type")

    async def heartbeat_loop(self, writer: asyncio.StreamWriter) -> None:
        while True:
            try:
                await asyncio.sleep(HEARTBEAT_INTERVAL)
                await self.send_status_update(writer, NodeStatus.RUNNING)
                print("Heartbeat sent")
            except Exception as e:
                print(f"Heartbeat failed: {e}")
                break

    async def read_loop(
        self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        while True:
            try:
                data = await reader.read(4096)
                if not data:
                    print("Connection closed by server")
                    break

                message = data.decode()
                print(f"Received message: {message}")

                try:
                    envelope = json.loads(message)
                    await self.handle_message(envelope, writer)
                except json.JSONDecodeError:
                    pass
            except Exception as e:
                print(f"Read error: {e}")
                break

    async def run(self) -> None:
        while True:
            try:
                reader, writer = await self.connect()

                # Register with server
                try:
                    await self.register(writer)
                except Exception as e:
                    print(f"Failed to register: {e}")
                    writer.close()
                    await writer.wait_closed()
                    await asyncio.sleep(RECONNECT_DELAY)
                    continue

                # Send initial status
                try:
                    await self.send_status_update(writer, NodeStatus.RUNNING)
                except Exception as e:
                    print(f"Failed to send initial status: {e}")
                    writer.close()
                    await writer.wait_closed()
                    await asyncio.sleep(RECONNECT_DELAY)
                    continue

                # Start heartbeat and read loops
                heartbeat_task = asyncio.create_task(self.heartbeat_loop(writer))
                read_task = asyncio.create_task(self.read_loop(reader, writer))

                # Wait for either task to complete (indicating an error)
                done, pending = await asyncio.wait(
                    [heartbeat_task, read_task], return_when=asyncio.FIRST_COMPLETED
                )

                # Cancel remaining tasks
                for task in pending:
                    task.cancel()

                writer.close()
                await writer.wait_closed()

            except Exception as e:
                print(f"Connection failed: {e}")

            print(f"Reconnecting in {RECONNECT_DELAY} seconds...")
            await asyncio.sleep(RECONNECT_DELAY)


async def main():
    print("=== Honeybee Node Example (Python) ===")

    # Configuration
    node_id = random.randint(0, 2**64 - 1)
    node_name = f"python-node-{node_id % 1000}"
    address = "0.0.0.0"
    port = 8080
    node_type = NodeType.AGENT
    server_address = os.getenv("SERVER_ADDRESS", "127.0.0.1:9001")

    print(f"Node ID: {node_id}")
    print(f"Node Name: {node_name}")
    print(f"Node Type: {node_type.value}")
    print(f"Server: {server_address}")
    print()

    client = NodeClient(node_id, node_name, address, port, node_type, server_address)
    await client.run()


if __name__ == "__main__":
    asyncio.run(main())
